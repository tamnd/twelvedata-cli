// Package twelvedata exposes the Twelve Data financial API as a kit Domain: a
// driver that a multi-domain host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/twelvedata-cli/twelvedata"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// twelvedata:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone twelvedata binary, so the binary and a host
// share one source of truth.
package twelvedata

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the Twelve Data driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "twelvedata",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "twelvedata",
			Short:  "CLI for Twelve Data financial market API",
			Long: `CLI for Twelve Data financial market API

twelvedata reads real-time and historical financial market data — stock prices,
quotes, OHLCV time series, stock listings, and market exchanges — over plain
HTTPS from api.twelvedata.com, shapes it into clean records, and prints output
that pipes into the rest of your tools.

The "demo" API key works for basic endpoints with AAPL and a limited set of
symbols. Register a free key at https://twelvedata.com/pricing for full access.`,
			Site: Host,
			Repo: "https://github.com/tamnd/twelvedata-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:     "price",
		Group:    "market",
		Single:   true,
		Summary:  "Get real-time price for a symbol",
		URIType:  "symbol",
		Resolver: true,
		Args:     []kit.Arg{{Name: "symbol", Help: "stock or forex symbol (e.g. AAPL, EUR/USD)"}},
	}, getPrice)

	kit.Handle(app, kit.OpMeta{
		Name:    "quote",
		Group:   "market",
		Single:  true,
		Summary: "Get full quote with daily OHLCV and 52-week range",
		Args:    []kit.Arg{{Name: "symbol", Help: "stock or forex symbol (e.g. AAPL)"}},
	}, getQuote)

	kit.Handle(app, kit.OpMeta{
		Name:    "timeseries",
		Group:   "market",
		List:    true,
		Summary: "Get OHLCV time series bars for a symbol",
		Args:    []kit.Arg{{Name: "symbol", Help: "stock or forex symbol (e.g. AAPL)"}},
	}, getTimeSeries)

	kit.Handle(app, kit.OpMeta{
		Name:    "stocks",
		Group:   "reference",
		List:    true,
		Summary: "List available stock symbols",
	}, listStocks)

	kit.Handle(app, kit.OpMeta{
		Name:    "exchanges",
		Group:   "reference",
		List:    true,
		Summary: "List available market exchanges",
	}, listExchanges)
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

// --- input structs ---

type priceInput struct {
	Symbol string  `kit:"arg" help:"stock or forex symbol (e.g. AAPL, EUR/USD)"`
	Key    string  `kit:"flag" name:"key" help:"Twelve Data API key (default: demo)"`
	Client *Client `kit:"inject"`
}

type quoteInput struct {
	Symbol string  `kit:"arg" help:"stock or forex symbol (e.g. AAPL)"`
	Key    string  `kit:"flag" name:"key" help:"Twelve Data API key (default: demo)"`
	Client *Client `kit:"inject"`
}

type timeSeriesInput struct {
	Symbol   string  `kit:"arg" help:"stock or forex symbol (e.g. AAPL)"`
	Interval string  `kit:"flag" help:"bar interval: 1min 5min 15min 30min 1h 4h 1day 1week 1month" default:"1day"`
	Count    int     `kit:"flag" help:"number of bars to return" default:"30"`
	Key      string  `kit:"flag" name:"key" help:"Twelve Data API key (default: demo)"`
	Client   *Client `kit:"inject"`
}

type stocksInput struct {
	Exchange string  `kit:"flag" help:"filter by exchange (e.g. NYSE)"`
	Country  string  `kit:"flag" help:"filter by country code (e.g. US)"`
	Limit    int     `kit:"flag,inherit" help:"max results" default:"20"`
	Key      string  `kit:"flag" name:"key" help:"Twelve Data API key (default: demo)"`
	Client   *Client `kit:"inject"`
}

type exchangesInput struct {
	Key    string  `kit:"flag" name:"key" help:"Twelve Data API key (default: demo)"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getPrice(ctx context.Context, in priceInput, emit func(*Price) error) error {
	applyKey(in.Client, in.Key)
	p, err := in.Client.GetPrice(ctx, in.Symbol)
	if err != nil {
		return err
	}
	return emit(p)
}

func getQuote(ctx context.Context, in quoteInput, emit func(*Quote) error) error {
	applyKey(in.Client, in.Key)
	q, err := in.Client.GetQuote(ctx, in.Symbol)
	if err != nil {
		return err
	}
	return emit(q)
}

func getTimeSeries(ctx context.Context, in timeSeriesInput, emit func(*Bar) error) error {
	applyKey(in.Client, in.Key)
	interval := in.Interval
	if interval == "" {
		interval = "1day"
	}
	count := in.Count
	if count <= 0 {
		count = 30
	}
	bars, err := in.Client.GetTimeSeries(ctx, in.Symbol, interval, count)
	if err != nil {
		return err
	}
	for i := range bars {
		if err := emit(&bars[i]); err != nil {
			return err
		}
	}
	return nil
}

func listStocks(ctx context.Context, in stocksInput, emit func(*Stock) error) error {
	applyKey(in.Client, in.Key)
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	stocks, err := in.Client.ListStocks(ctx, in.Exchange, in.Country, limit)
	if err != nil {
		return err
	}
	for i := range stocks {
		if err := emit(&stocks[i]); err != nil {
			return err
		}
	}
	return nil
}

func listExchanges(ctx context.Context, in exchangesInput, emit func(*Exchange) error) error {
	applyKey(in.Client, in.Key)
	exchanges, err := in.Client.ListExchanges(ctx)
	if err != nil {
		return err
	}
	for i := range exchanges {
		if err := emit(&exchanges[i]); err != nil {
			return err
		}
	}
	return nil
}

// applyKey updates the client's API key from the --key flag if provided.
func applyKey(c *Client, key string) {
	if key != "" {
		c.cfg.APIKey = key
	}
}

// --- Resolver (URI driver) ---

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty twelvedata reference")
	}
	return "symbol", strings.ToUpper(input), nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "symbol":
		return "https://api.twelvedata.com/price?symbol=" + id + "&apikey=demo", nil
	default:
		return "", errs.Usage("twelvedata has no resource type %q", uriType)
	}
}
