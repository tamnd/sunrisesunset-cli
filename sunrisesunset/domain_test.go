package sunrisesunset

import (
	"testing"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// The client's HTTP behaviour is covered in sunrisesunset_test.go.

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
	typ, id, err := Domain{}.Classify("51.5074,-0.1278")
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if typ != "location" {
		t.Errorf("type = %q, want location", typ)
	}
	if id != "51.5074,-0.1278" {
		t.Errorf("id = %q", id)
	}
}

func TestClassify_empty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("expected error on empty input, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("location", "51.5074,-0.1278")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}

func TestLocate_badType(t *testing.T) {
	_, err := Domain{}.Locate("page", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
