package twelvedata

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "twelvedata" {
		t.Errorf("Scheme = %q, want twelvedata", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "twelvedata" {
		t.Errorf("Identity.Binary = %q, want twelvedata", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"AAPL", "symbol", "AAPL"},
		{"aapl", "symbol", "AAPL"},
		{"EUR/USD", "symbol", "EUR/USD"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := Domain{}.Classify("")
	if err == nil {
		t.Error("Classify(\"\") expected error, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("symbol", "AAPL")
	if err != nil {
		t.Fatalf("Locate error: %v", err)
	}
	want := "https://api.twelvedata.com/price?symbol=AAPL&apikey=demo"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate with unknown type expected error, got nil")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Price{Symbol: "AAPL", Price: "291.13"}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "twelvedata://symbol/AAPL"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("twelvedata", "MSFT")
	if err != nil || got.String() != "twelvedata://symbol/MSFT" {
		t.Errorf("ResolveOn = (%q, %v), want twelvedata://symbol/MSFT", got.String(), err)
	}
}
