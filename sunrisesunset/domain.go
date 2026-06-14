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

// Domain is the sunrisesunset driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "sunrisesunset",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "sunrisesunset",
			Short:  "A command line for the Sunrise-Sunset API.",
			Long: `A command line for the Sunrise-Sunset API.

sunrisesunset fetches sunrise and sunset times for any coordinates via
https://api.sunrise-sunset.org. No API key required.`,
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
	}, lookup)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
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
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// lookupInput holds the arguments and flags for the lookup command.
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

// Classify turns any accepted input into the canonical (type, id).
// For sunrisesunset a URI looks like sunrisesunset://suntimes/<lat>,<lng>.
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("unrecognized sunrisesunset reference: %q", input)
	}
	return "suntimes", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "suntimes" {
		return "", errs.Usage("sunrisesunset has no resource type %q", uriType)
	}
	return fmt.Sprintf("%s/json?lat=%s&formatted=0", BaseURL, id), nil
}

// mapErr converts a library error into the kit error kind that carries the right
// exit code.
func mapErr(err error) error {
	return err
}
