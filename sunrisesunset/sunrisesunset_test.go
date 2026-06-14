package sunrisesunset_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/sunrisesunset-cli/sunrisesunset"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *sunrisesunset.Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := sunrisesunset.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return sunrisesunset.NewClient(cfg)
}

const sampleResponse = `{
	"status":"OK",
	"results":{
		"sunrise":"2026-06-14T03:44:32+00:00",
		"sunset":"2026-06-14T19:40:02+00:00",
		"solar_noon":"2026-06-14T11:42:17+00:00",
		"day_length":57630,
		"civil_twilight_begin":"2026-06-14T03:13:59+00:00",
		"civil_twilight_end":"2026-06-14T20:10:35+00:00",
		"nautical_twilight_begin":"2026-06-14T02:32:27+00:00",
		"nautical_twilight_end":"2026-06-14T20:51:47+00:00",
		"astronomical_twilight_begin":"2026-06-14T01:40:52+00:00",
		"astronomical_twilight_end":"2026-06-14T21:43:42+00:00"
	}
}`

func TestSun_userAgent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("request carried no User-Agent header")
		}
		if !strings.Contains(ua, "sunrisesunset-cli") {
			t.Errorf("User-Agent %q does not contain sunrisesunset-cli", ua)
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	_, err := c.Sun(context.Background(), 51.5074, -0.1278, "2026-06-14")
	if err != nil {
		t.Fatal(err)
	}
}

func TestSun_parseSunriseAndSunset(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleResponse))
	})
	st, err := c.Sun(context.Background(), 51.5074, -0.1278, "2026-06-14")
	if err != nil {
		t.Fatal(err)
	}
	if st.Sunrise != "2026-06-14T03:44:32+00:00" {
		t.Errorf("Sunrise = %q, want 2026-06-14T03:44:32+00:00", st.Sunrise)
	}
	if st.Sunset != "2026-06-14T19:40:02+00:00" {
		t.Errorf("Sunset = %q, want 2026-06-14T19:40:02+00:00", st.Sunset)
	}
	if st.DayLength != 57630 {
		t.Errorf("DayLength = %d, want 57630", st.DayLength)
	}
}

func TestSun_dateParamInURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date != "2026-06-14" {
			t.Errorf("date query param = %q, want 2026-06-14", date)
		}
		lat := r.URL.Query().Get("lat")
		if lat == "" {
			t.Error("missing lat query param")
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	_, err := c.Sun(context.Background(), 48.8566, 2.3522, "2026-06-14")
	if err != nil {
		t.Fatal(err)
	}
}

func TestSun_retry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer ts.Close()

	cfg := sunrisesunset.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := sunrisesunset.NewClient(cfg)

	st, err := c.Sun(context.Background(), 0, 0, "2026-06-14")
	if err != nil {
		t.Fatal(err)
	}
	if st.DayLength != 57630 {
		t.Errorf("DayLength = %d, want 57630", st.DayLength)
	}
	if hits != 3 {
		t.Errorf("server hits = %d, want 3", hits)
	}
}

func TestSun_errorStatus(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"INVALID_REQUEST","results":{}}`))
	})
	_, err := c.Sun(context.Background(), 0, 0, "bad-date")
	if err == nil {
		t.Fatal("expected error on INVALID_REQUEST status, got nil")
	}
	if !strings.Contains(err.Error(), "INVALID_REQUEST") {
		t.Errorf("error %q does not mention INVALID_REQUEST", err.Error())
	}
}

func TestSun_latLngInURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		lng := r.URL.Query().Get("lng")
		if lng == "" {
			t.Error("missing lng query param")
		}
		formatted := r.URL.Query().Get("formatted")
		if formatted != "0" {
			t.Errorf("formatted = %q, want 0", formatted)
		}
		_, _ = w.Write([]byte(sampleResponse))
	})
	_, err := c.Sun(context.Background(), 35.6762, 139.6503, "2026-06-14")
	if err != nil {
		t.Fatal(err)
	}
}
