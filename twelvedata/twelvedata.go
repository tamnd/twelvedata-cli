// Package twelvedata is the library behind the twelvedata command line:
// the HTTP client, request shaping, and typed data models for the Twelve Data
// financial market API at https://api.twelvedata.com.
//
// The API requires an API key appended as ?apikey={key} to every request.
// The "demo" key works for basic endpoints with limited symbols. Register a
// free key at https://twelvedata.com/pricing for broader access.
package twelvedata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the Twelve Data API hostname.
const Host = "api.twelvedata.com"

// DefaultUserAgent identifies the client to the API.
const DefaultUserAgent = "twelvedata-cli/0.1 (tamnd87@gmail.com)"

// Config holds constructor parameters for Client.
type Config struct {
	BaseURL   string
	UserAgent string
	APIKey    string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.twelvedata.com",
		APIKey:    "demo",
		Rate:      1 * time.Second,
		Timeout:   15 * time.Second,
		Retries:   3,
		UserAgent: DefaultUserAgent,
	}
}

// Client is the Twelve Data HTTP client.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient constructs a Client from cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// --- Public output types ---

// Price is a real-time price record for a symbol.
type Price struct {
	Symbol string `kit:"id" json:"symbol"`
	Price  string `json:"price"`
}

// Quote is a full daily quote with OHLCV and 52-week range.
type Quote struct {
	Symbol        string `kit:"id" json:"symbol"`
	Name          string `json:"name"`
	Exchange      string `json:"exchange"`
	Date          string `json:"date"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Close         string `json:"close"`
	Volume        string `json:"volume"`
	Change        string `json:"change"`
	ChangePercent string `json:"change_percent"`
	Week52Low     string `json:"week52_low"`
	Week52High    string `json:"week52_high"`
}

// Bar is one OHLCV candle in a time series.
type Bar struct {
	Datetime string `kit:"id" json:"datetime"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Close    string `json:"close"`
	Volume   string `json:"volume"`
}

// Stock is an available stock listing.
type Stock struct {
	Symbol   string `kit:"id" json:"symbol"`
	Name     string `json:"name"`
	Exchange string `json:"exchange"`
	Country  string `json:"country"`
	Currency string `json:"currency"`
	Type     string `json:"type"`
}

// Exchange is a market exchange.
type Exchange struct {
	Code     string `kit:"id" json:"code"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Country  string `json:"country"`
	Timezone string `json:"timezone"`
}

// --- Wire types (API JSON shapes) ---

type wireQuote struct {
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	Exchange      string `json:"exchange"`
	Datetime      string `json:"datetime"`
	Open          string `json:"open"`
	High          string `json:"high"`
	Low           string `json:"low"`
	Close         string `json:"close"`
	Volume        string `json:"volume"`
	Change        string `json:"change"`
	PercentChange string `json:"percent_change"`
	FiftyTwoWeek  struct {
		Low  string `json:"low"`
		High string `json:"high"`
	} `json:"fifty_two_week"`
}

type wireTimeSeries struct {
	Meta struct {
		Symbol   string `json:"symbol"`
		Interval string `json:"interval"`
	} `json:"meta"`
	Values []struct {
		Datetime string `json:"datetime"`
		Open     string `json:"open"`
		High     string `json:"high"`
		Low      string `json:"low"`
		Close    string `json:"close"`
		Volume   string `json:"volume"`
	} `json:"values"`
}

type wireStocks struct {
	Data []struct {
		Symbol   string `json:"symbol"`
		Name     string `json:"name"`
		Currency string `json:"currency"`
		Exchange string `json:"exchange"`
		Country  string `json:"country"`
		Type     string `json:"type"`
	} `json:"data"`
	Count  int    `json:"count"`
	Status string `json:"status"`
}

type wireExchanges struct {
	Data []struct {
		Title    string `json:"title"`
		Name     string `json:"name"`
		Code     string `json:"code"`
		Country  string `json:"country"`
		Timezone string `json:"timezone"`
	} `json:"data"`
	Status string `json:"status"`
}

// --- Client methods ---

// GetPrice fetches a real-time price for the given symbol.
func (c *Client) GetPrice(ctx context.Context, symbol string) (*Price, error) {
	u := c.buildURL("/price", "symbol", symbol)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w struct {
		Price string `json:"price"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("price decode: %w", err)
	}
	if w.Price == "" {
		return nil, fmt.Errorf("no price returned for symbol %q", symbol)
	}
	return &Price{Symbol: symbol, Price: w.Price}, nil
}

// GetQuote fetches a full quote (OHLCV + 52-week range) for the given symbol.
func (c *Client) GetQuote(ctx context.Context, symbol string) (*Quote, error) {
	u := c.buildURL("/quote", "symbol", symbol)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w wireQuote
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("quote decode: %w", err)
	}
	if w.Symbol == "" {
		return nil, fmt.Errorf("no quote returned for symbol %q", symbol)
	}
	return &Quote{
		Symbol:        w.Symbol,
		Name:          w.Name,
		Exchange:      w.Exchange,
		Date:          w.Datetime,
		Open:          w.Open,
		High:          w.High,
		Low:           w.Low,
		Close:         w.Close,
		Volume:        w.Volume,
		Change:        w.Change,
		ChangePercent: w.PercentChange,
		Week52Low:     w.FiftyTwoWeek.Low,
		Week52High:    w.FiftyTwoWeek.High,
	}, nil
}

// GetTimeSeries fetches OHLCV bars for the given symbol, interval, and count.
// interval is one of: 1min, 5min, 15min, 30min, 1h, 4h, 1day, 1week, 1month.
func (c *Client) GetTimeSeries(ctx context.Context, symbol, interval string, count int) ([]Bar, error) {
	countStr := fmt.Sprintf("%d", count)
	u := c.buildURL("/time_series", "symbol", symbol, "interval", interval, "outputsize", countStr)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w wireTimeSeries
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("time_series decode: %w", err)
	}
	bars := make([]Bar, len(w.Values))
	for i, v := range w.Values {
		bars[i] = Bar{
			Datetime: v.Datetime,
			Open:     v.Open,
			High:     v.High,
			Low:      v.Low,
			Close:    v.Close,
			Volume:   v.Volume,
		}
	}
	return bars, nil
}

// ListStocks returns available stock symbols, optionally filtered by exchange and country.
// limit caps the results client-side.
func (c *Client) ListStocks(ctx context.Context, exchange, country string, limit int) ([]Stock, error) {
	params := []string{}
	if exchange != "" {
		params = append(params, "exchange", exchange)
	}
	if country != "" {
		params = append(params, "country", country)
	}
	u := c.buildURL("/stocks", params...)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w wireStocks
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("stocks decode: %w", err)
	}
	data := w.Data
	if limit > 0 && len(data) > limit {
		data = data[:limit]
	}
	out := make([]Stock, len(data))
	for i, s := range data {
		out[i] = Stock{
			Symbol:   s.Symbol,
			Name:     s.Name,
			Exchange: s.Exchange,
			Country:  s.Country,
			Currency: s.Currency,
			Type:     s.Type,
		}
	}
	return out, nil
}

// ListExchanges returns all available market exchanges.
func (c *Client) ListExchanges(ctx context.Context) ([]Exchange, error) {
	u := c.buildURL("/exchanges")
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var w wireExchanges
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("exchanges decode: %w", err)
	}
	out := make([]Exchange, len(w.Data))
	for i, e := range w.Data {
		out[i] = Exchange{
			Code:     e.Code,
			Name:     e.Name,
			Title:    e.Title,
			Country:  e.Country,
			Timezone: e.Timezone,
		}
	}
	return out, nil
}

// buildURL assembles a full API URL with the apikey parameter and optional
// additional key=value pairs. Pairs with empty values are skipped.
func (c *Client) buildURL(path string, params ...string) string {
	apiKey := c.cfg.APIKey
	if apiKey == "" {
		apiKey = "demo"
	}
	base := c.cfg.BaseURL
	if base == "" {
		base = "https://api.twelvedata.com"
	}
	u := base + path + "?apikey=" + url.QueryEscape(apiKey)
	for i := 0; i+1 < len(params); i += 2 {
		if params[i+1] != "" {
			u += "&" + params[i] + "=" + url.QueryEscape(params[i+1])
		}
	}
	return u
}

// get fetches a URL with pacing and retries. The body is fully read and closed.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	ua := c.cfg.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
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
