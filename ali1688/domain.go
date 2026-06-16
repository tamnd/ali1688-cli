package ali1688

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the 1688.com kit driver.
type Domain struct{}

// Info describes the scheme and identity for the ali1688 driver.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "ali1688",
		Hosts:  []string{Host, SearchHost, DetailHost},
		Identity: kit.Identity{
			Binary: "ali1688",
			Short:  "Browse 1688.com wholesale products",
			Long: `ali1688 reads public 1688.com data over HTTPS.

NOTE: 1688.com blocks datacenter IPs with anti-bot checks. Commands exit 5
when the block fires. Run from a residential or mobile IP for best results.

Quick start:
  ali1688 search laptop             search wholesale laptops
  ali1688 search phone --min-price 50 --max-price 200
  ali1688 product 789123456         product details by offer ID`,
			Site: Host,
			Repo: "https://github.com/tamnd/ali1688-cli",
		},
	}
}

// Register installs the client factory and the two operations onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "catalog",
		Summary: "Search wholesale products on 1688.com",
		Args:    []kit.Arg{{Name: "query", Help: "search keywords"}},
	}, searchOp)

	kit.Handle(app, kit.OpMeta{
		Name:     "product",
		Group:    "catalog",
		Single:   true,
		Resolver: true,
		URIType:  "product",
		Summary:  "Fetch product details by offer ID",
		Args:     []kit.Arg{{Name: "id", Help: "1688 offer ID (numeric)"}},
	}, productOp)
}

func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient(DefaultConfig())
	if cfg.UserAgent != "" {
		c.cfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.cfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.cfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.cfg.Timeout = cfg.Timeout
		c.http.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- input structs ---

type searchInput struct {
	Query    string  `kit:"arg" help:"search keywords"`
	MinPrice float64 `kit:"flag" name:"min-price" help:"minimum price (CNY)"`
	MaxPrice float64 `kit:"flag" name:"max-price" help:"maximum price (CNY)"`
	Limit    int     `kit:"flag,inherit" help:"max results"`
	Client   *Client `kit:"inject"`
}

type productInput struct {
	ID     string  `kit:"arg" help:"1688 offer ID"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(*SearchResult) error) error {
	results, err := in.Client.Search(ctx, in.Query, SearchOptions{
		MinPrice: in.MinPrice,
		MaxPrice: in.MaxPrice,
		Limit:    in.Limit,
	})
	if err != nil {
		return mapErr(err)
	}
	for i := range results {
		if err := emit(&results[i]); err != nil {
			return err
		}
	}
	return nil
}

func productOp(ctx context.Context, in productInput, emit func(*Product) error) error {
	p, err := in.Client.GetProduct(ctx, in.ID)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

// --- Resolver ---

var (
	offerIDFromPathRE = regexp.MustCompile(`/offer/(\d+)\.html`)
	bareNumericRE     = regexp.MustCompile(`^\d+$`)
)

// Classify turns an offer URL or bare numeric ID into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if u, err2 := url.Parse(input); err2 == nil &&
		(u.Scheme == "http" || u.Scheme == "https") {
		if m := offerIDFromPathRE.FindStringSubmatch(u.Path); m != nil {
			return "product", m[1], nil
		}
	}
	if bareNumericRE.MatchString(strings.TrimSpace(input)) {
		return "product", strings.TrimSpace(input), nil
	}
	return "", "", errs.Usage("unrecognized 1688 reference: %q", input)
}

// Locate returns the canonical HTTPS URL for (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "product":
		return BaseDetailURL + "/offer/" + id + ".html", nil
	}
	return "", errs.Usage("ali1688 has no resource type %q", uriType)
}

// mapErr converts library errors to kit error kinds with the right exit codes.
func mapErr(err error) error {
	switch {
	case isBlockedErr(err):
		return errs.RateLimited("%s", err.Error())
	case isNotFoundErr(err):
		return errs.NotFound("%s", err.Error())
	}
	return err
}

func isBlockedErr(err error) bool {
	return err != nil && (err == ErrBlocked || strings.Contains(err.Error(), "blocked"))
}

func isNotFoundErr(err error) bool {
	return err == ErrNotFound
}
