package twelvedata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testClient returns a Client pointed at the given test server with no pacing.
func testClient(srv *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 0
	cfg.Timeout = 5 * time.Second
	return NewClient(cfg)
}

func TestGetPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/price" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		sym := r.URL.Query().Get("symbol")
		if sym != "AAPL" {
			t.Errorf("symbol = %q, want AAPL", sym)
		}
		if r.URL.Query().Get("apikey") == "" {
			t.Error("apikey missing from request")
		}
		_, _ = w.Write([]byte(`{"price":"291.13"}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	p, err := c.GetPrice(context.Background(), "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if p.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", p.Symbol)
	}
	if p.Price != "291.13" {
		t.Errorf("Price = %q, want 291.13", p.Price)
	}
}

func TestGetQuote(t *testing.T) {
	wire := map[string]any{
		"symbol":          "AAPL",
		"name":            "Apple Inc.",
		"exchange":        "NASDAQ",
		"datetime":        "2024-12-16",
		"open":            "295.00",
		"high":            "296.19",
		"low":             "290.27",
		"close":           "291.13000",
		"volume":          "52184786",
		"change":          "-4.5",
		"percent_change":  "-1.52217",
		"fifty_two_week":  map[string]string{"low": "169.21", "high": "260.10"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/quote" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer srv.Close()

	c := testClient(srv)
	q, err := c.GetQuote(context.Background(), "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if q.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want AAPL", q.Symbol)
	}
	if q.Name != "Apple Inc." {
		t.Errorf("Name = %q, want Apple Inc.", q.Name)
	}
	if q.Exchange != "NASDAQ" {
		t.Errorf("Exchange = %q, want NASDAQ", q.Exchange)
	}
	if q.Close != "291.13000" {
		t.Errorf("Close = %q, want 291.13000", q.Close)
	}
	if q.Week52Low != "169.21" {
		t.Errorf("Week52Low = %q, want 169.21", q.Week52Low)
	}
	if q.Week52High != "260.10" {
		t.Errorf("Week52High = %q, want 260.10", q.Week52High)
	}
	if q.ChangePercent != "-1.52217" {
		t.Errorf("ChangePercent = %q, want -1.52217", q.ChangePercent)
	}
}

func TestGetTimeSeries(t *testing.T) {
	wire := map[string]any{
		"meta": map[string]any{
			"symbol":   "AAPL",
			"interval": "1day",
		},
		"values": []map[string]any{
			{"datetime": "2024-12-16", "open": "280.00", "high": "282.00", "low": "275.00", "close": "280.00", "volume": "12345678"},
			{"datetime": "2024-12-13", "open": "275.00", "high": "279.00", "low": "273.00", "close": "278.00", "volume": "9876543"},
			{"datetime": "2024-12-12", "open": "270.00", "high": "276.00", "low": "269.00", "close": "274.00", "volume": "8765432"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time_series" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.URL.Query().Get("interval") != "1day" {
			t.Errorf("interval = %q, want 1day", r.URL.Query().Get("interval"))
		}
		if r.URL.Query().Get("outputsize") != "3" {
			t.Errorf("outputsize = %q, want 3", r.URL.Query().Get("outputsize"))
		}
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer srv.Close()

	c := testClient(srv)
	bars, err := c.GetTimeSeries(context.Background(), "AAPL", "1day", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(bars) != 3 {
		t.Fatalf("len(bars) = %d, want 3", len(bars))
	}
	if bars[0].Datetime != "2024-12-16" {
		t.Errorf("bars[0].Datetime = %q, want 2024-12-16", bars[0].Datetime)
	}
	if bars[0].Close != "280.00" {
		t.Errorf("bars[0].Close = %q, want 280.00", bars[0].Close)
	}
}

func TestListStocks(t *testing.T) {
	wire := map[string]any{
		"data": []map[string]any{
			{"symbol": "A", "name": "Agilent Technologies Inc.", "currency": "USD", "exchange": "NYSE", "country": "United States", "type": "Common Stock"},
			{"symbol": "AA", "name": "Alcoa Corp", "currency": "USD", "exchange": "NYSE", "country": "United States", "type": "Common Stock"},
			{"symbol": "AAPL", "name": "Apple Inc.", "currency": "USD", "exchange": "NASDAQ", "country": "United States", "type": "Common Stock"},
		},
		"count":  3,
		"status": "ok",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stocks" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer srv.Close()

	c := testClient(srv)

	// test without limit cap
	stocks, err := c.ListStocks(context.Background(), "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks) != 3 {
		t.Errorf("len(stocks) = %d, want 3", len(stocks))
	}
	if stocks[0].Symbol != "A" {
		t.Errorf("stocks[0].Symbol = %q, want A", stocks[0].Symbol)
	}

	// test with client-side limit
	stocks2, err := c.ListStocks(context.Background(), "", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(stocks2) != 2 {
		t.Errorf("len(stocks2) = %d, want 2 (limit applied)", len(stocks2))
	}
}

func TestListStocksWithFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("exchange") != "NYSE" {
			t.Errorf("exchange = %q, want NYSE", r.URL.Query().Get("exchange"))
		}
		if r.URL.Query().Get("country") != "US" {
			t.Errorf("country = %q, want US", r.URL.Query().Get("country"))
		}
		_, _ = w.Write([]byte(`{"data":[],"count":0,"status":"ok"}`))
	}))
	defer srv.Close()

	c := testClient(srv)
	_, err := c.ListStocks(context.Background(), "NYSE", "US", 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListExchanges(t *testing.T) {
	wire := map[string]any{
		"data": []map[string]any{
			{"title": "Australian Securities Exchange", "name": "ASX", "code": "XASX", "country": "Australia", "timezone": "Australia/Sydney"},
			{"title": "NASDAQ", "name": "NASDAQ", "code": "XNAS", "country": "United States", "timezone": "America/New_York"},
		},
		"status": "ok",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/exchanges" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(wire)
	}))
	defer srv.Close()

	c := testClient(srv)
	exchanges, err := c.ListExchanges(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(exchanges) != 2 {
		t.Fatalf("len(exchanges) = %d, want 2", len(exchanges))
	}
	if exchanges[0].Code != "XASX" {
		t.Errorf("exchanges[0].Code = %q, want XASX", exchanges[0].Code)
	}
	if exchanges[0].Country != "Australia" {
		t.Errorf("exchanges[0].Country = %q, want Australia", exchanges[0].Country)
	}
	if exchanges[1].Name != "NASDAQ" {
		t.Errorf("exchanges[1].Name = %q, want NASDAQ", exchanges[1].Name)
	}
}

func TestGetRetries(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"price":"100.00"}`))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.Timeout = 5 * time.Second
	c := NewClient(cfg)

	p, err := c.GetPrice(context.Background(), "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if p.Price != "100.00" {
		t.Errorf("Price = %q after retries", p.Price)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestBuildURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.twelvedata.com"
	cfg.APIKey = "testkey"
	c := NewClient(cfg)

	u := c.buildURL("/price", "symbol", "AAPL")
	if u != "https://api.twelvedata.com/price?apikey=testkey&symbol=AAPL" {
		t.Errorf("buildURL = %q", u)
	}

	// empty value params are skipped
	u2 := c.buildURL("/stocks", "exchange", "", "country", "US")
	if u2 != "https://api.twelvedata.com/stocks?apikey=testkey&country=US" {
		t.Errorf("buildURL with empty param = %q", u2)
	}
}
