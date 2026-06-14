package sunrisesunset

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeResponse builds a minimal valid API response for the given day_length.
func fakeResponse(dayLength int) []byte {
	type results struct {
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
	}
	type resp struct {
		Results results `json:"results"`
		Status  string  `json:"status"`
	}
	b, _ := json.Marshal(resp{
		Results: results{
			Sunrise:            "2026-06-14T09:22:37+00:00",
			Sunset:             "2026-06-15T00:30:08+00:00",
			SolarNoon:          "2026-06-14T16:56:22+00:00",
			DayLength:          dayLength,
			CivilTwilightBegin: "2026-06-14T08:54:18+00:00",
			CivilTwilightEnd:   "2026-06-15T00:58:27+00:00",
		},
		Status: "OK",
	})
	return b
}

func TestLookupBasic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		q := r.URL.Query()
		if q.Get("lat") != "40.7128" {
			t.Errorf("lat = %q, want 40.7128", q.Get("lat"))
		}
		if q.Get("lng") != "-74.0060" {
			t.Errorf("lng = %q, want -74.0060", q.Get("lng"))
		}
		if q.Get("formatted") != "0" {
			t.Errorf("formatted = %q, want 0", q.Get("formatted"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakeResponse(54451))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Timeout = 5 * time.Second
	// point at test server
	origBase := BaseURL
	_ = origBase
	// We call Get directly via Lookup, so override the client's base by patching
	// the URL in a subtest wrapper. Easiest: use a custom transport.
	c.HTTP.Transport = rewriteTransport(srv.URL)

	st, err := c.Lookup(context.Background(), "40.7128", "-74.0060", "today")
	if err != nil {
		t.Fatal(err)
	}
	if st.Lat != "40.7128" {
		t.Errorf("Lat = %q, want 40.7128", st.Lat)
	}
	if st.Lng != "-74.0060" {
		t.Errorf("Lng = %q, want -74.0060", st.Lng)
	}
	if st.Sunrise != "2026-06-14T09:22:37+00:00" {
		t.Errorf("Sunrise = %q", st.Sunrise)
	}
	if st.DayLengthHours != "15.13" {
		t.Errorf("DayLengthHours = %q, want 15.13", st.DayLengthHours)
	}
}

func TestLookupWithDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("date") != "2026-06-21" {
			t.Errorf("date = %q, want 2026-06-21", r.URL.Query().Get("date"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakeResponse(36000))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = rewriteTransport(srv.URL)

	st, err := c.Lookup(context.Background(), "35.6895", "139.6917", "2026-06-21")
	if err != nil {
		t.Fatal(err)
	}
	if st.Date != "2026-06-21" {
		t.Errorf("Date = %q, want 2026-06-21", st.Date)
	}
	if st.DayLengthHours != "10.00" {
		t.Errorf("DayLengthHours = %q, want 10.00", st.DayLengthHours)
	}
}

func TestLookupDefaultsToToday(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("date") != "today" {
			t.Errorf("date = %q, want today", r.URL.Query().Get("date"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fakeResponse(40000))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = rewriteTransport(srv.URL)

	st, err := c.Lookup(context.Background(), "-33.8688", "151.2093", "")
	if err != nil {
		t.Fatal(err)
	}
	if st.Date != "today" {
		t.Errorf("Date = %q, want today", st.Date)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestLookupAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":{},"status":"INVALID_REQUEST"}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.HTTP.Transport = rewriteTransport(srv.URL)

	_, err := c.Lookup(context.Background(), "999", "999", "today")
	if err == nil {
		t.Fatal("expected error for INVALID_REQUEST, got nil")
	}
}

// rewriteTransport returns an http.RoundTripper that rewrites the host of every
// request to the given base URL, so the client code uses BaseURL normally but
// hits the test server.
type hostRewriter struct {
	base string
}

func (h hostRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.URL.Scheme = "http"
	r2.URL.Host = h.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(r2)
}

func rewriteTransport(base string) http.RoundTripper {
	return hostRewriter{base: base}
}
