// Package ali1688 is the library behind the ali1688 command line:
// the HTTP client, pacing, bot-detection, and typed data models for 1688.com.
//
// 1688.com is Alibaba's domestic B2B wholesale marketplace. It deploys
// Alibaba's anti-bot stack (Tengine + Arachnid). Datacenter IPs are blocked
// with HTTP 403 or CAPTCHA pages. The client detects these patterns and
// returns ErrBlocked, which the CLI maps to exit code 5.
package ali1688

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent mimics a real browser to reduce bot-detection friction.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Host is the primary hostname for 1688.com.
const Host = "1688.com"

// SearchHost is the search subdomain.
const SearchHost = "s.1688.com"

// DetailHost is the product detail subdomain.
const DetailHost = "detail.1688.com"

// BaseSearchURL is the search endpoint root.
const BaseSearchURL = "https://" + SearchHost

// BaseDetailURL is the product detail endpoint root.
const BaseDetailURL = "https://" + DetailHost

// ErrBlocked is returned when the site's anti-bot stack triggers.
// The CLI maps this to exit code 5.
var ErrBlocked = errors.New("blocked by anti-bot (exit 5)")

// ErrNotFound is returned when a product does not exist.
var ErrNotFound = errors.New("product not found")

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for 1688.com.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseSearchURL,
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client is a rate-limited, bot-detection-aware HTTP client for 1688.com.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Get fetches rawURL and returns the response body.
// It paces requests, retries on transient errors, and returns ErrBlocked
// when the anti-bot stack fires.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	return c.do(ctx, rawURL)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		c.pace()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.cfg.UserAgent)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, rerr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if rerr != nil {
			lastErr = rerr
			continue
		}
		if isBlocked(resp, body) {
			return nil, ErrBlocked
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("http %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, fmt.Errorf("after %d attempts: %w", c.cfg.Retries, lastErr)
}

// pace blocks until at least Rate has elapsed since the last request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

// isBlocked reports whether resp/body indicate an anti-bot block.
func isBlocked(resp *http.Response, body []byte) bool {
	if resp.StatusCode == http.StatusForbidden {
		return true
	}
	if resp.StatusCode == http.StatusFound {
		loc := resp.Header.Get("Location")
		if strings.Contains(loc, "login.taobao.com") ||
			strings.Contains(loc, "login.1688.com") {
			return true
		}
	}
	s := string(body)
	return strings.Contains(s, "J_Captcha") ||
		strings.Contains(s, "slidecaptcha") ||
		strings.Contains(s, "nc_csdk") ||
		strings.Contains(s, "访问被拒绝") ||
		strings.Contains(s, "Access Denied") ||
		strings.Contains(s, "sCaptchaToken") ||
		strings.Contains(s, "block_page")
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
