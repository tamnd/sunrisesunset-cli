// Package sunrisesunset is the library behind the sunrisesunset command line:
// the HTTP client, request shaping, and the typed data models for the
// sunrise-sunset.org API (sunrise/sunset/twilight times for any location).
//
// The Client is the spine every command shares. It sets a real User-Agent,
// paces requests so a busy session stays polite, and retries the transient
// failures (429 and 5xx) that any public API throws under load.
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

// Host is the API hostname this package talks to.
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
		UserAgent: "sunrisesunset-cli/0.1.0 (github.com/tamnd/sunrisesunset-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// SunTimes holds the full set of times returned by the API for one query.
type SunTimes struct {
	Location               string `json:"location"                kit:"id"` // "lat,lon" formatted as "%.4f,%.4f"
	Date                   string `json:"date"`
	Timezone               string `json:"timezone"`
	Sunrise                string `json:"sunrise"`
	Sunset                 string `json:"sunset"`
	SolarNoon              string `json:"solar_noon"`
	DayLengthSeconds       int    `json:"day_length_seconds"`
	CivilTwilightBegin     string `json:"civil_twilight_begin"`
	CivilTwilightEnd       string `json:"civil_twilight_end"`
	NauticalTwilightBegin  string `json:"nautical_twilight_begin"`
	NauticalTwilightEnd    string `json:"nautical_twilight_end"`
	AstronomicalTwilightBegin string `json:"astronomical_twilight_begin"`
	AstronomicalTwilightEnd   string `json:"astronomical_twilight_end"`
}

// Client talks to api.sunrise-sunset.org over HTTP.
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

// apiResults is the raw "results" object inside the JSON envelope.
type apiResults struct {
	Sunrise                   string `json:"sunrise"`
	Sunset                    string `json:"sunset"`
	SolarNoon                 string `json:"solar_noon"`
	DayLength                 int    `json:"day_length"`
	CivilTwilightBegin        string `json:"civil_twilight_begin"`
	CivilTwilightEnd          string `json:"civil_twilight_end"`
	NauticalTwilightBegin     string `json:"nautical_twilight_begin"`
	NauticalTwilightEnd       string `json:"nautical_twilight_end"`
	AstronomicalTwilightBegin string `json:"astronomical_twilight_begin"`
	AstronomicalTwilightEnd   string `json:"astronomical_twilight_end"`
}

// apiResponse is the raw JSON envelope.
type apiResponse struct {
	Status  string     `json:"status"`
	Results apiResults `json:"results"`
	Tzid    string     `json:"tzid"`
}

// Sun fetches sunrise/sunset times for the given latitude, longitude, and date.
// date must be in "YYYY-MM-DD" format; pass empty string for today.
// tz is an IANA timezone name (e.g. "America/New_York"); pass empty string for UTC.
func (c *Client) Sun(ctx context.Context, lat, lon float64, date, tz string) (*SunTimes, error) {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	u := fmt.Sprintf("%s/json?lat=%f&lng=%f&formatted=0&date=%s", c.cfg.BaseURL, lat, lon, date)
	if tz != "" {
		u += "&tzid=" + tz
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var r apiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("sunrisesunset: decode response: %w", err)
	}
	if r.Status != "OK" {
		return nil, fmt.Errorf("sunrisesunset: API status %q", r.Status)
	}
	timezone := r.Tzid
	if timezone == "" {
		timezone = "UTC"
	}
	st := &SunTimes{
		Location:                  fmt.Sprintf("%.4f,%.4f", lat, lon),
		Date:                      date,
		Timezone:                  timezone,
		Sunrise:                   r.Results.Sunrise,
		Sunset:                    r.Results.Sunset,
		SolarNoon:                 r.Results.SolarNoon,
		DayLengthSeconds:          r.Results.DayLength,
		CivilTwilightBegin:        r.Results.CivilTwilightBegin,
		CivilTwilightEnd:          r.Results.CivilTwilightEnd,
		NauticalTwilightBegin:     r.Results.NauticalTwilightBegin,
		NauticalTwilightEnd:       r.Results.NauticalTwilightEnd,
		AstronomicalTwilightBegin: r.Results.AstronomicalTwilightBegin,
		AstronomicalTwilightEnd:   r.Results.AstronomicalTwilightEnd,
	}
	return st, nil
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
	return nil, fmt.Errorf("sunrisesunset: get %s: %w", u, lastErr)
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
