// Package sunrisesunset is the library behind the sunrisesunset command line:
// the HTTP client, request shaping, and the typed data models for the
// Sunrise-Sunset API (https://api.sunrise-sunset.org).
//
// The Client paces and retries requests to stay polite. Build your endpoint
// calls and JSON decoding on top of it.
package sunrisesunset

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Host is the API host this client talks to.
const Host = "api.sunrise-sunset.org"

// Config holds all tuneable parameters for a Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns the production configuration for api.sunrise-sunset.org.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.sunrise-sunset.org",
		UserAgent: "sunrisesunset-cli/0.1 (tamnd87@gmail.com)",
		Rate:      200 * time.Millisecond,
		Timeout:   10 * time.Second,
		Retries:   3,
	}
}

// Client talks to the Sunrise-Sunset API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// apiResponse is the raw JSON shape the API returns.
type apiResponse struct {
	Results struct {
		Sunrise            string `json:"sunrise"`
		Sunset             string `json:"sunset"`
		SolarNoon          string `json:"solar_noon"`
		DayLength          int    `json:"day_length"`
		CivilTwilightBegin string `json:"civil_twilight_begin"`
		CivilTwilightEnd   string `json:"civil_twilight_end"`
	} `json:"results"`
	Status string `json:"status"`
}

// Lookup fetches sunrise/sunset times for the given coordinates and date.
// lat and lng are decimal degree strings (e.g. "40.7128", "-74.0060").
// date is "YYYY-MM-DD" or "today".
func (c *Client) Lookup(ctx context.Context, lat, lng, date string) (*SunTimes, error) {
	if date == "" {
		date = "today"
	}
	u := fmt.Sprintf("%s/json?lat=%s&lng=%s&formatted=0&date=%s", c.cfg.BaseURL, lat, lng, date)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.Status != "OK" {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	r := resp.Results
	return &SunTimes{
		Lat:                lat,
		Lng:                lng,
		Date:               date,
		Sunrise:            r.Sunrise,
		Sunset:             r.Sunset,
		SolarNoon:          r.SolarNoon,
		DayLengthHours:     fmt.Sprintf("%.2f", float64(r.DayLength)/3600),
		CivilTwilightBegin: r.CivilTwilightBegin,
		CivilTwilightEnd:   r.CivilTwilightEnd,
	}, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
