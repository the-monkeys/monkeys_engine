package snapshot

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

//go:embed assets/logo_master.png
var logoMasterPNG []byte

// FrameLayout describes the output canvas and the position/size of the video area within it.
type FrameLayout struct {
	FrameW, FrameH     int // total output dimensions (even integers)
	VidX, VidY         int // top-left corner of the video area within the frame
	VidAreaW, VidAreaH int // dimensions of the video area (even integers)
}

// frameDimensions calculates the card layout dimensions for a given video size.
// All returned values are clamped to even integers for h264 compatibility.
func frameDimensions(videoW, videoH int) FrameLayout {
	// Border: ~0.5% of width, minimum 4px.
	borderPx := maxInt(4, videoW*5/1000)
	borderPx = evenClamp(borderPx)

	// Use the longer dimension so portrait/vertical videos get proportionally
	// sized bars instead of the tiny ones that result from using width alone.
	refDim := maxInt(videoW, videoH)

	// Top bar: 7% of reference dimension.
	topBarH := evenClamp(refDim * 7 / 100)

	// Bottom bar: 11% of reference dimension.
	bottomBarH := evenClamp(refDim * 11 / 100)

	// Ensure minimum bar heights for readability.
	if topBarH < 56 {
		topBarH = 56
	}
	if bottomBarH < 80 {
		bottomBarH = 80
	}

	outputW := evenClamp(videoW + 2*borderPx)
	outputH := evenClamp(borderPx + topBarH + videoH + bottomBarH + borderPx)

	vidX := borderPx
	vidY := borderPx + topBarH

	return FrameLayout{
		FrameW:   outputW,
		FrameH:   outputH,
		VidX:     vidX,
		VidY:     vidY,
		VidAreaW: evenClamp(videoW),
		VidAreaH: evenClamp(videoH),
	}
}

// evenClamp rounds down to the nearest even integer.
func evenClamp(v int) int {
	return v &^ 1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// renderVideoFrame renders the complete card frame PNG at the output resolution.
// The video area is filled white — the caller overlays the actual video via FFmpeg.
// Returns the FrameLayout describing where to place the video.
func renderVideoFrame(videoW, videoH int, displayName, handle, accentHex, outPath string) (FrameLayout, error) {
	layout := frameDimensions(videoW, videoH)

	dc := gg.NewContext(layout.FrameW, layout.FrameH)

	// 1. Fill entire canvas with black (this creates the border).
	dc.SetColor(color.Black)
	dc.Clear()

	// 2. Fill the inner area with white (inside the border).
	borderPx := layout.VidX // borderPx == VidX since vidX = borderPx
	innerX := float64(borderPx)
	innerY := float64(borderPx)
	innerW := float64(layout.FrameW - 2*borderPx)
	innerH := float64(layout.FrameH - 2*borderPx)
	dc.SetColor(color.White)
	dc.DrawRectangle(innerX, innerY, innerW, innerH)
	dc.Fill()

	// 3. Calculate bar regions.
	topBarH := layout.VidY - borderPx
	topBarY := float64(borderPx)
	bottomBarY := float64(layout.VidY + layout.VidAreaH)
	bottomBarH := float64(layout.FrameH-borderPx) - bottomBarY
	contentW := innerW // width of drawable area inside borders

	// =========== TOP BAR ===========
	// Logo on the left.
	logoImg, err := png.Decode(bytes.NewReader(logoMasterPNG))
	if err != nil {
		return layout, fmt.Errorf("decode logo: %w", err)
	}

	logoPad := float64(topBarH) * 0.20 // padding around logo
	logoMaxH := int(float64(topBarH) - 2*logoPad)
	logoMaxW := logoMaxH // square logo

	scaledLogo := scaleImageFit(logoImg, logoMaxW, logoMaxH)
	logoDrawW := scaledLogo.Bounds().Dx()
	logoDrawH := scaledLogo.Bounds().Dy()

	logoX := int(innerX + logoPad*1.5)
	logoY := int(topBarY + (float64(topBarH)-float64(logoDrawH))/2)
	dc.DrawImage(scaledLogo, logoX, logoY)

	// "monkeys.com.co." text after logo — vertically centered with the logo.
	domainText := "monkeys.com.co."
	domainPt := math.Round(float64(topBarH) * 0.25)
	domainFace, dferr := makeFontFace(gobold.TTF, domainPt)
	if dferr == nil {
		dc.SetFontFace(domainFace)
		defer domainFace.Close()
	}
	domainX := float64(logoX+logoDrawW) + logoPad*0.8
	// Anchor text to the logo's vertical center (not the bar center).
	logoCenterY := float64(logoY) + float64(logoDrawH)/2.0
	dc.SetColor(color.Black)
	dc.DrawStringAnchored(domainText, domainX, logoCenterY, 0, 0.4)

	// "STUDIO" label on the right — same vertical center as logo.
	studioText := "STUDIO"
	studioPt := math.Round(float64(topBarH) * 0.25)
	studioFace, sferr := makeFontFace(goregular.TTF, studioPt)
	if sferr == nil {
		dc.SetFontFace(studioFace)
		defer studioFace.Close()
	}
	studioX := innerX + contentW - logoPad*1.5
	dc.SetRGBA(0.3, 0.3, 0.3, 1) // dark gray
	dc.DrawStringAnchored(studioText, studioX, logoCenterY, 1, 0.4)

	// =========== BOTTOM BAR ===========
	// Usable area starts after a top margin and ends before the border.
	sepMargin := bottomBarH * 0.10
	separatorY := bottomBarY + sepMargin

	// Separator line.
	dc.SetRGBA(0.82, 0.82, 0.82, 1)
	dc.SetLineWidth(math.Max(1.5, float64(layout.FrameW)*0.002))
	dc.DrawLine(innerX+logoPad, separatorY, innerX+contentW-logoPad, separatorY)
	dc.Stroke()

	// Usable bottom area: between separator and bottom border.
	usableTop := separatorY + sepMargin*0.5
	usableBottom := float64(layout.FrameH-borderPx) - sepMargin*0.5
	usableH := usableBottom - usableTop

	// Avatar circle with accent colour + initials.
	accent := parseHexColor(accentHex)
	ar := float64(accent.R) / 255
	ag := float64(accent.G) / 255
	ab := float64(accent.B) / 255

	avatarD := usableH * 0.55
	avatarR := avatarD / 2
	avatarCX := innerX + logoPad*1.5 + avatarR
	avatarCY := usableTop + usableH/2

	dc.SetRGBA(ar, ag, ab, 1)
	dc.DrawCircle(avatarCX, avatarCY, avatarR)
	dc.Fill()

	// Initials inside avatar.
	initials := getInitials(displayName, handle)
	initialsPt := math.Round(avatarR * 0.75)
	initFace, iferr := makeFontFace(gobold.TTF, initialsPt)
	if iferr == nil {
		dc.SetFontFace(initFace)
		defer initFace.Close()
	}
	dc.SetColor(color.White)
	dc.DrawStringAnchored(initials, avatarCX, avatarCY, 0.5, 0.5)

	// Text column: to the right of the avatar.
	textX := avatarCX + avatarR + logoPad

	// Compute text sizes so we can center the name+handle block with the avatar.
	namePt := math.Round(usableH * 0.28)
	handlePt := math.Round(usableH * 0.20)
	lineGap := usableH * 0.06 // gap between name and handle
	totalTextH := namePt + lineGap + handlePt

	// Position the text block so its vertical center == avatar center.
	blockTop := avatarCY - totalTextH/2

	// Display name — bold black (top line).
	nameFace, nferr := makeFontFace(gobold.TTF, namePt)
	if nferr == nil {
		dc.SetFontFace(nameFace)
		defer nameFace.Close()
	}
	nameStr := strings.TrimSpace(displayName)
	if nameStr == "" {
		nameStr = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	}
	// Name: top of text at blockTop → anchor ay=0 means text descends from this Y.
	dc.SetColor(color.Black)
	dc.DrawStringAnchored(nameStr, textX, blockTop, 0, 0)

	// Handle — regular, gray (second line below name).
	handleFace, hferr := makeFontFace(goregular.TTF, handlePt)
	if hferr == nil {
		dc.SetFontFace(handleFace)
		defer handleFace.Close()
	}
	handleStr := strings.TrimSpace(handle)
	if handleStr != "" && !strings.HasPrefix(handleStr, "@") {
		handleStr = "@" + handleStr
	}
	// Handle: starts below name + gap.
	handleTopY := blockTop + namePt + lineGap
	dc.SetRGBA(0.4, 0.4, 0.4, 1)
	dc.DrawStringAnchored(handleStr, textX, handleTopY, 0, 0)

	if err := dc.SavePNG(outPath); err != nil {
		return layout, fmt.Errorf("save frame png: %w", err)
	}
	return layout, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// parseHexColor parses a CSS hex colour string. Returns brand orange on failure.
func parseHexColor(s string) color.RGBA {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 6 {
		var r, g, b uint8
		if n, _ := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); n == 3 {
			return color.RGBA{R: r, G: g, B: b, A: 255}
		}
	}
	return color.RGBA{R: 255, G: 85, B: 66, A: 255} // #FF5542
}

// getInitials extracts up to 2 uppercase initials from a display name,
// falling back to the first two characters of the handle.
func getInitials(displayName, handle string) string {
	name := strings.TrimSpace(displayName)
	if name != "" {
		parts := strings.Fields(name)
		if len(parts) >= 2 && len(parts[0]) > 0 && len(parts[1]) > 0 {
			return strings.ToUpper(string([]byte{parts[0][0], parts[1][0]}))
		}
		if len(name) >= 2 {
			return strings.ToUpper(name[:2])
		}
	}
	h := strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if len(h) >= 2 {
		return strings.ToUpper(h[:2])
	}
	return "TM"
}

// scaleImageFit downscales src to fit within maxW×maxH preserving aspect ratio.
// Uses Catmull-Rom resampling. Never upscales.
func scaleImageFit(src image.Image, maxW, maxH int) image.Image {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return src
	}
	ratio := math.Min(float64(maxW)/float64(sw), float64(maxH)/float64(sh))
	if ratio >= 1.0 {
		return src
	}
	nw := int(math.Round(float64(sw) * ratio))
	nh := int(math.Round(float64(sh) * ratio))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

// makeFontFace parses a TTF byte slice and returns a font.Face at the given point size.
func makeFontFace(ttfBytes []byte, points float64) (xfont.Face, error) {
	f, err := opentype.Parse(ttfBytes)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    points,
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
}
