package snapshot

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
)

const maxDurationSec = 600.0 // 10-minute hard cap

// RenderRequest is the JSON body for the X-video branded render endpoint.
type RenderRequest struct {
	TweetID      string `json:"tweetId"`
	VideoURL     string `json:"videoUrl"`
	AuthorName   string `json:"authorName"`
	AuthorHandle string `json:"authorHandle"`
	AccentColor  string `json:"accentColor"`
}

type SnapshotService struct {
	logger *zap.SugaredLogger
	config *config.Config
}

func NewSnapshotService(cfg *config.Config, log *zap.SugaredLogger) *SnapshotService {
	return &SnapshotService{logger: log, config: cfg}
}

func RegisterRoutes(router *gin.Engine, cfg *config.Config, log *zap.SugaredLogger) {
	svc := NewSnapshotService(cfg, log)
	router.POST("/api/snapshot/tweet/video/render", svc.HandleVideoRender)
	router.POST("/api/snapshot/video/brand", svc.HandleLocalVideoBrand)
}

// HandleVideoRender downloads an X-post video and brands it with the card frame.
func (s *SnapshotService) HandleVideoRender(c *gin.Context) {
	var req RenderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.VideoURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "videoUrl is required"})
		return
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFmpeg not available"})
		return
	}

	tempDir := filepath.Join(os.TempDir(), "monkeys-render-"+uuid.NewString())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "source.mp4")
	if err := downloadFile(req.VideoURL, videoPath); err != nil {
		s.logger.Errorf("download video: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch source video"})
		return
	}

	// Guard: reject audio-only files.
	if !probeHasVideoStream(videoPath) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "File contains no video stream"})
		return
	}

	// Guard: reject videos longer than 10 minutes.
	if dur, derr := probeDuration(videoPath); derr == nil && dur > maxDurationSec {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Video exceeds 10 minute limit"})
		return
	}

	outputPath := filepath.Join(tempDir, "output.mp4")
	if err := s.encodeFramed(videoPath, outputPath, req.AuthorName, req.AuthorHandle, req.AccentColor); err != nil {
		s.logger.Errorf("encode framed video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Video rendering failed"})
		return
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="x-post-%s-branded.mp4"`, req.TweetID))
	c.File(outputPath)
}

// HandleLocalVideoBrand accepts a multipart video upload and brands it with the card frame.
func (s *SnapshotService) HandleLocalVideoBrand(c *gin.Context) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "FFmpeg not available"})
		return
	}

	const maxUpload = 500 << 20
	if err := c.Request.ParseMultipartForm(maxUpload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large or invalid form data"})
		return
	}

	videoFile, _, err := c.Request.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing video file"})
		return
	}
	defer videoFile.Close()

	authorName := c.Request.FormValue("authorName")
	authorHandle := c.Request.FormValue("authorHandle")
	accentColor := c.Request.FormValue("accentColor")

	tempDir := filepath.Join(os.TempDir(), "monkeys-brand-"+uuid.NewString())
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workspace"})
		return
	}
	defer os.RemoveAll(tempDir)

	videoPath := filepath.Join(tempDir, "source.mp4")
	vf, err := os.Create(videoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save video"})
		return
	}
	if _, err := io.Copy(vf, videoFile); err != nil {
		vf.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write video"})
		return
	}
	vf.Close()

	// Guard: reject audio-only files.
	if !probeHasVideoStream(videoPath) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "File contains no video stream"})
		return
	}

	// Guard: reject videos longer than 10 minutes.
	if dur, derr := probeDuration(videoPath); derr == nil && dur > maxDurationSec {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Video exceeds 10 minute limit"})
		return
	}

	outputPath := filepath.Join(tempDir, "output.mp4")
	if err := s.encodeFramed(videoPath, outputPath, authorName, authorHandle, accentColor); err != nil {
		s.logger.Errorf("brand local video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Video branding failed"})
		return
	}

	c.Header("Content-Type", "video/mp4")
	c.Header("Content-Disposition", `attachment; filename="branded-video.mp4"`)
	c.File(outputPath)
}

// encodeFramed is the core pipeline:
// 1. Probe video dimensions (with rotation handling).
// 2. Render the card frame PNG at the correct output resolution.
// 3. FFmpeg: frame as background, video overlaid at the correct position.
func (s *SnapshotService) encodeFramed(videoPath, outputPath, authorName, authorHandle, accentColor string) error {
	videoW, videoH, probeErr := probeVideoSize(videoPath)
	if probeErr != nil {
		return fmt.Errorf("probe video size: %w (file may not contain a valid video stream)", probeErr)
	}

	// Handle iPhone rotation metadata: swap dimensions if rotated 90/270.
	rot := probeRotation(videoPath)
	if rot == 90 || rot == 270 {
		videoW, videoH = videoH, videoW
	}

	// Render the card frame PNG.
	tempDir := filepath.Dir(outputPath)
	framePath := filepath.Join(tempDir, "frame.png")
	layout, err := renderVideoFrame(videoW, videoH, authorName, authorHandle, accentColor, framePath)
	if err != nil {
		return fmt.Errorf("render video frame: %w", err)
	}

	// Build FFmpeg filter: frame is base, video is overlaid at (VidX, VidY).
	// Scale video to exact video-area dimensions to handle any rounding.
	filter := fmt.Sprintf(
		"[0:v]scale=%d:%d:flags=lanczos[vid];[1:v][vid]overlay=%d:%d:format=auto,format=yuv420p[out]",
		layout.VidAreaW, layout.VidAreaH, layout.VidX, layout.VidY,
	)

	args := []string{
		"-nostdin", "-hide_banner", "-loglevel", "warning",
		"-threads", "2", "-filter_threads", "2",
		"-i", videoPath,
		"-i", framePath,
		"-vsync", "cfr",
		"-filter_complex", filter,
		"-map", "[out]",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "20",
		"-pix_fmt", "yuv420p",
		"-x264-params", "threads=2:me=hex:ref=2:rc-lookahead=20",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y", outputPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Env = append(os.Environ(), "OMP_NUM_THREADS=2")
	out, execErr := cmd.CombinedOutput()
	if execErr != nil {
		tail := string(out)
		if len(tail) > 2000 {
			tail = tail[len(tail)-2000:]
		}
		return fmt.Errorf("ffmpeg encode: %w\n%s", execErr, tail)
	}
	return nil
}

// ─── Probe Helpers ──────────────────────────────────────────────────────────

func probeVideoSize(path string) (width, height int, err error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		path,
	).Output()
	if err != nil {
		return 0, 0, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected ffprobe output: %q", string(out))
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("parse ffprobe dimensions: %q", string(out))
	}
	return w, h, nil
}

func probeDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, err
	}
	var d float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(out)), "%f", &d)
	return d, err
}

// probeRotation returns the video stream's rotation in degrees (0, 90, 180, 270).
// iPhones store portrait video as landscape with rotation=90 side data.
func probeRotation(path string) int {
	out, _ := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream_side_data=rotation",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	s := strings.TrimSpace(string(out))
	if s == "" {
		// Try the older metadata tag location.
		out2, _ := exec.Command("ffprobe",
			"-v", "error",
			"-select_streams", "v:0",
			"-show_entries", "stream_tags=rotate",
			"-of", "default=noprint_wrappers=1:nokey=1",
			path,
		).Output()
		s = strings.TrimSpace(string(out2))
	}
	if s == "" {
		return 0
	}
	r, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	r = ((r % 360) + 360) % 360
	return r
}

// probeHasVideoStream returns true if the file contains at least one video stream.
func probeHasVideoStream(path string) bool {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		path,
	).Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(string(out)), "video")
}

// ─── Download Helper ────────────────────────────────────────────────────────

func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
