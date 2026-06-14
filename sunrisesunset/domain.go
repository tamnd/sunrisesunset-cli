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
			Long: `sunrisesunset fetches sunrise, sunset, solar noon, day length, and civil,
nautical, and astronomical twilight times for any latitude/longitude pair.

It calls the free sunrise-sunset.org API (no key required) and prints a JSON
record for the requested location and date. Times are RFC 3339 strings.`,
			Site: Host,
			Repo: "https://github.com/tamnd/sunrisesunset-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "sun",
		Group:   "read",
		Single:  true,
		Summary: "Get sunrise and sunset times for a location",
		URIType: "location",
	}, getSun)
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

// --- inputs ---

// sunInput holds the flags for the sun command. Both --lat and --lon are named
// float64 flags (not positional args) because cobra treats tokens starting with
// "-" as flags. A negative longitude like -74.0060 would be misread as a flag
// if passed positionally. Named flags avoid that ambiguity.
type sunInput struct {
	Lat    float64 `kit:"flag" help:"latitude in decimal degrees (e.g. --lat 40.7128 or --lat -33.8688)"`
	Lon    float64 `kit:"flag" help:"longitude in decimal degrees (e.g. --lon -74.0060 or --lon 151.2093)"`
	Date   string  `kit:"flag" help:"date in YYYY-MM-DD format (default: today)"`
	Tz     string  `kit:"flag" help:"IANA timezone name (e.g. America/New_York); default UTC"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getSun(ctx context.Context, in sunInput, emit func(*SunTimes) error) error {
	t, err := in.Client.Sun(ctx, in.Lat, in.Lon, in.Date, in.Tz)
	if err != nil {
		return mapErr(err)
	}
	return emit(t)
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
