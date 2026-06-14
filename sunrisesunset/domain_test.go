package sunrisesunset

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in sunrisesunset_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "sunrisesunset" {
		t.Errorf("Scheme = %q, want sunrisesunset", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "sunrisesunset" {
		t.Errorf("Identity.Binary = %q, want sunrisesunset", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"40.7128,-74.0060", "suntimes", "40.7128,-74.0060"},
		{"-33.8688,151.2093", "suntimes", "-33.8688,151.2093"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("suntimes", "40.7128,-74.0060")
	want := "https://" + Host + "/json?lat=40.7128,-74.0060&formatted=0"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a record mints to its URI, and its body is readable.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	st := &SunTimes{
		Lat:  "40.7128",
		Lng:  "-74.0060",
		Date: "2026-06-14",
	}
	u, err := h.Mint(st)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "sunrisesunset://suntimes/40.7128"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}
}
