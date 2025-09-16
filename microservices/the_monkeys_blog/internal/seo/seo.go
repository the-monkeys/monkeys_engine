package seo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/the-monkeys/the_monkeys/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type SEOManager interface {
	HandleSEOForBlog(ctx context.Context, blogId, slug string) error
	ShareBlogToTelegram(ctx context.Context, blogId, slug string) error
	ShareBlogToDiscord(ctx context.Context, blogId, slug string) error
}

type seoManager struct {
	log    *zap.SugaredLogger
	config *config.Config
}

func NewSEOManager(log *zap.SugaredLogger, config *config.Config) SEOManager {
	return &seoManager{
		log:    log,
		config: config,
	}
}

type urlNotification struct {
	Type       string `json:"type,omitempty"`
	Url        string `json:"url,omitempty"`
	NotifyTime string `json:"notifyTime,omitempty"`
}

type urlNotificationsMetadata struct {
	Url          string           `json:"url,omitempty"`
	LatestUpdate *urlNotification `json:"latestUpdate,omitempty"`
	LatestRemove *urlNotification `json:"latestRemove,omitempty"`
}

func (s *seoManager) HandleSEOForBlog(ctx context.Context, blogId, slug string) error {
	if !s.config.SEO.Enabled {
		s.log.Debugw("SEO disabled", "blog_id", blogId)
		return nil
	}
	s.log.Debugw("handling SEO", "blog_id", blogId, "slug", slug)

	// Use a detached context with timeout so it isn't cancelled when the request returns
	cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Prepare the URL for the Google Indexing API
	indexingURL := s.config.SEO.GoogleIndexingAPI
	blogURL := fmt.Sprintf("%s/blog/%s", s.config.SEO.BaseURL, slug)

	fmt.Printf("blogURL: %v\n", blogURL)

	// Create the request payload
	payload := map[string]interface{}{
		"url":  blogURL,
		"type": "URL_UPDATED",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		s.log.Errorf("failed to marshal SEO payload for blog %s: %v", blogId, err)
		return fmt.Errorf("failed to marshal SEO payload: %w", err)
	}

	// Authenticate using the service account (read JSON file path from config)
	credBytes, err := os.ReadFile(s.config.SEO.GoogleCredentialsFile)
	if err != nil {
		s.log.Errorf("failed to read SEO credentials file '%s' for blog %s: %v", s.config.SEO.GoogleCredentialsFile, blogId, err)
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	creds, err := google.CredentialsFromJSON(cctx, credBytes, "https://www.googleapis.com/auth/indexing")
	if err != nil {
		s.log.Errorf("failed to create credentials from JSON for blog %s: %v", blogId, err)
		return fmt.Errorf("failed to create credentials: %w", err)
	}

	client := oauth2.NewClient(cctx, creds.TokenSource)
	client.Timeout = 15 * time.Second

	// Create a new HTTP request
	req, err := http.NewRequestWithContext(cctx, "POST", indexingURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		s.log.Errorf("failed to create SEO request for blog %s: %v", blogId, err)
		return fmt.Errorf("failed to create SEO request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		s.log.Errorf("failed to send SEO request for blog %s: %v", blogId, err)
		return fmt.Errorf("failed to send SEO request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.log.Errorf("SEO request for blog %s failed with status: %s, body: %s", blogId, resp.Status, string(body))
		return fmt.Errorf("SEO request failed with status: %s", resp.Status)
	}

	s.log.Infof("Successfully submitted blog %s for SEO indexing.", blogId)

	// Verify via metadata endpoint asynchronously
	// go s.verifyURLIndexing(blogURL)

	// Additionally, you might want to ping sitemaps or other services.
	if s.config.SEO.SearchConsoleSitemapURL != "" {
		// Ping search console that sitemap has been updated.
		// This is usually done by sending a GET request to a specific URL.
		sitemapURL := fmt.Sprintf("https://www.google.com/ping?sitemap=%s", s.config.SEO.SearchConsoleSitemapURL)
		_, err := http.Get(sitemapURL)
		if err != nil {
			s.log.Warnf("Failed to ping sitemap for blog %s: %v", blogId, err)
		} else {
			s.log.Infof("Successfully pinged sitemap for blog %s.", blogId)
		}
	}

	return nil
}

func (s *seoManager) verifyURLIndexing(blogURL string) {
	// Try a few times with backoff; metadata may not be available immediately after publish
	delays := []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}

	for attempt, d := range delays {
		time.Sleep(d)

		vctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		credBytes, err := os.ReadFile(s.config.SEO.GoogleCredentialsFile)
		if err != nil {
			s.log.Warnf("[SEO verify] cannot read creds: %v", err)
			return
		}

		creds, err := google.CredentialsFromJSON(vctx, credBytes, "https://www.googleapis.com/auth/indexing")
		if err != nil {
			s.log.Warnf("[SEO verify] cannot create creds: %v", err)
			return
		}

		client := oauth2.NewClient(vctx, creds.TokenSource)
		client.Timeout = 10 * time.Second

		metaURL := "https://indexing.googleapis.com/v3/urlNotifications/metadata?url=" + url.QueryEscape(blogURL)
		req, err := http.NewRequestWithContext(vctx, http.MethodGet, metaURL, nil)
		if err != nil {
			s.log.Warnf("[SEO verify] cannot create request: %v", err)
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			s.log.Warnf("[SEO verify] request failed (attempt %d/%d): %v", attempt+1, len(delays), err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var meta urlNotificationsMetadata
			if err := json.Unmarshal(body, &meta); err != nil {
				s.log.Warnf("[SEO verify] cannot parse metadata: %v", err)
				return
			}
			if meta.LatestUpdate != nil {
				s.log.Infof("[SEO verify] LatestUpdate: %s at %s for %s", meta.LatestUpdate.Type, meta.LatestUpdate.NotifyTime, meta.Url)
			} else if meta.LatestRemove != nil {
				s.log.Infof("[SEO verify] LatestRemove: %s at %s for %s", meta.LatestRemove.Type, meta.LatestRemove.NotifyTime, meta.Url)
			} else {
				s.log.Infof("[SEO verify] Metadata returned but no updates/removes yet for %s", blogURL)
			}
			return
		}

		if resp.StatusCode == http.StatusNotFound {
			s.log.Warnf("[SEO verify] metadata not ready yet (404). Will retry after %s (attempt %d/%d)", d, attempt+1, len(delays))
			continue
		}

		s.log.Warnf("[SEO verify] non-200: %s body=%s", resp.Status, string(body))
		return
	}

	s.log.Warnf("[SEO verify] metadata still not available after retries for %s", blogURL)
}

func (s *seoManager) ShareBlogToTelegram(ctx context.Context, blogId, slug string) error {
	// Placeholder implementation
	s.log.Infof("Sharing blog %s to Telegram with slug: %s", blogId, slug)
	// Implement actual Telegram sharing logic here
	return nil
}

func (s *seoManager) ShareBlogToDiscord(ctx context.Context, blogId, slug string) error {
	// Placeholder implementation
	s.log.Infof("Sharing blog %s to Discord with slug: %s", blogId, slug)
	// Implement actual Discord sharing logic here
	return nil
}
