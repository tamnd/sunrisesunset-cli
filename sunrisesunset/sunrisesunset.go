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
	"time"
)

// DefaultUserAgent identifies the client to the API.
const DefaultUserAgent = "sunrisesunset-cli/0.1 (tamnd87@gmail.com)"

// Host is the API host this client talks to.
const Host = "api.sunrise-sunset.org"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to the Sunrise-Sunset API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 10s timeout, a 200ms
// minimum gap between requests, and three retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 10 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
	}
}

// apiResponse is the raw JSON shape the API returns.
type apiResponse struct {
	Results struct {
		Sunrise                string `json:"sunrise"`
		Sunset                 string `json:"sunset"`
		SolarNoon              string `json:"solar_noon"`
		DayLength              int    `json:"day_length"`
		CivilTwilightBegin     string `json:"civil_twilight_begin"`
		CivilTwilightEnd       string `json:"civil_twilight_end"`
		NauticalTwilightBegin  string `json:"nautical_twilight_begin"`
		NauticalTwilightEnd    string `json:"nautical_twilight_end"`
		AstroTwilightBegin     string `json:"astronomical_twilight_begin"`
		AstroTwilightEnd       string `json:"astronomical_twilight_end"`
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
	rawURL := fmt.Sprintf("%s/json?lat=%s&lng=%s&formatted=0&date=%s", BaseURL, lat, lng, date)
	body, err := c.Get(ctx, rawURL)
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

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
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
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
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
