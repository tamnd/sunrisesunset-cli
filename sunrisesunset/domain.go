package sunrisesunset

import (
	"context"
	"fmt"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes sunrisesunset as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/sunrisesunset-cli/sunrisesunset"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// sunrisesunset:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone sunrisesunset binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the sunrisesunset driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "sunrisesunset",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "sunrisesunset",
			Short:  "Get sunrise and sunset times for any location.",
			Long: `sunrisesunset fetches sunrise, sunset, solar noon, day length, and civil
twilight times for any latitude/longitude pair.

It calls the free sunrise-sunset.org API (no key required) and prints a JSON
record for the requested location and date. Times are in RFC 3339 UTC.`,
			Site: Host,
			Repo: "https://github.com/tamnd/sunrisesunset-cli",
		},
	}
}

// SunTimes is one record: the solar event times for a given location and date.
type SunTimes struct {
	Lat                string `kit:"id" json:"lat" table:"lat"`
	Lng                string `json:"lng" table:"lng"`
	Date               string `json:"date" table:"date"`
	Sunrise            string `json:"sunrise" table:"sunrise"`
	Sunset             string `json:"sunset" table:"sunset"`
	SolarNoon          string `json:"solar_noon" table:"solar_noon"`
	DayLengthHours     string `json:"day_length_hours" table:"day_length_hours"`
	CivilTwilightBegin string `json:"civil_twilight_begin" table:"civil_twilight_begin"`
	CivilTwilightEnd   string `json:"civil_twilight_end" table:"civil_twilight_end"`
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "lookup",
		Group:   "read",
		Single:  true,
		Summary: "Get sunrise and sunset times for a location",
		URIType: "location",
	}, lookup)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// lookupInput holds the flags for the lookup command.
// Both lat and lng are flags rather than positional args because cobra treats
// tokens starting with "-" as flags. A negative coordinate like -74.0060 or
// -33.8688 would be misread as a flag if passed positionally. Using --lat and
// --lng avoids that ambiguity.
type lookupInput struct {
	Lat    string  `kit:"flag" help:"latitude in decimal degrees (e.g. --lat 40.7128 or --lat -33.8688)"`
	Lng    string  `kit:"flag" help:"longitude in decimal degrees (e.g. --lng -74.0060 or --lng 151.2093)"`
	Date   string  `kit:"flag" help:"date in YYYY-MM-DD format (default: today)"`
	Client *Client `kit:"inject"`
}

func lookup(ctx context.Context, in lookupInput, emit func(*SunTimes) error) error {
	if in.Lat == "" {
		return errs.Usage("--lat is required")
	}
	if in.Lng == "" {
		return errs.Usage("--lng is required")
	}
	date := in.Date
	if date == "" {
		date = "today"
	}
	result, err := in.Client.Lookup(ctx, in.Lat, in.Lng, date)
	if err != nil {
		return mapErr(err)
	}
	return emit(result)
}

// Classify turns a coordinate pair into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("unrecognized sunrisesunset reference: %q", input)
	}
	return "location", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "location" {
		return "", errs.Usage("sunrisesunset has no resource type %q", uriType)
	}
	return fmt.Sprintf("https://%s/json?formatted=0&date=today&lat=%s", Host, id), nil
}

func mapErr(err error) error {
	return err
}
