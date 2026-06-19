package ali1688

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(baseURL string) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0 // no pacing in tests
	cfg.Retries = 2
	cfg.Timeout = 5 * time.Second
	return NewClient(cfg)
}

func TestGetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, "<html><body>ok</body></html>")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	body, err := c.Get(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Error("expected non-empty body")
	}
}

func TestGetBlocked403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, "<html><title>403 Forbidden</title></html>")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked, got %v", err)
	}
}

func TestGetBlockedCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body><div id="J_Captcha"></div></body></html>`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for CAPTCHA, got %v", err)
	}
}

func TestGetBlockedSlideCaptcha(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body><script>var slidecaptcha = true;</script></body></html>`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for slidecaptcha, got %v", err)
	}
}

func TestGetBlockedChineseMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "<html><title>访问被拒绝</title></html>")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Get(context.Background(), srv.URL+"/test")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked for 访问被拒绝, got %v", err)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, "<html><body>recovered</body></html>")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cfg.Retries = 5
	body, err := c.Get(context.Background(), srv.URL+"/test")
	if err != nil {
		t.Fatal(err)
	}
	if string(body) == "" {
		t.Error("expected non-empty body after retry")
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestSearchBlockedReturnsErrBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cfg.BaseURL = srv.URL
	_, err := c.Search(context.Background(), "laptop", SearchOptions{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked from Search, got %v", err)
	}
}

func TestSearchParsesJSONInScriptTag(t *testing.T) {
	body := `<html><body>
<script>
window.__INIT_DATA__ = {
  "offerList": [
    {"offerId": 123, "subject": "USB-C Adapter", "priceInfo": {"price": "5.5", "priceTo": "8.0"},
     "tradeQuantity": "100 pcs", "companyName": "Shenzhen Co", "province": "Guangdong",
     "detailPageUrl": "https://detail.1688.com/offer/123.html"}
  ]
};
</script></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cfg.BaseURL = srv.URL
	results, err := c.Search(context.Background(), "usb", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Title != "USB-C Adapter" {
		t.Errorf("Title = %q, want USB-C Adapter", r.Title)
	}
	if r.PriceMin != 5.5 {
		t.Errorf("PriceMin = %v, want 5.5", r.PriceMin)
	}
	if r.PriceMax != 8.0 {
		t.Errorf("PriceMax = %v, want 8.0", r.PriceMax)
	}
	if r.Supplier != "Shenzhen Co" {
		t.Errorf("Supplier = %q, want Shenzhen Co", r.Supplier)
	}
}

func TestSearchPriceFilter(t *testing.T) {
	body := `<html><body><script>
window.__INIT_DATA__ = {
  "offerList": [
    {"offerId": 1, "subject": "Cheap Item", "priceInfo": {"price": "1.0", "priceTo": "2.0"}, "companyName": "Co1", "province": "GD"},
    {"offerId": 2, "subject": "Mid Item", "priceInfo": {"price": "10.0", "priceTo": "15.0"}, "companyName": "Co2", "province": "SH"},
    {"offerId": 3, "subject": "Expensive Item", "priceInfo": {"price": "100.0", "priceTo": "200.0"}, "companyName": "Co3", "province": "BJ"}
  ]
};
</script></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.cfg.BaseURL = srv.URL
	results, err := c.Search(context.Background(), "item", SearchOptions{MinPrice: 5, MaxPrice: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 filtered result, got %d: %+v", len(results), results)
	}
	if results[0].Title != "Mid Item" {
		t.Errorf("filtered result = %q, want Mid Item", results[0].Title)
	}
}

func TestProductBlockedReturnsErrBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	// Patch so GetProduct calls srv.URL instead of detail.1688.com
	_, err := c.do(context.Background(), srv.URL+"/offer/123.html")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected ErrBlocked from do, got %v", err)
	}
}

func TestIsBlocked(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		want    bool
	}{
		{"403", 403, "", true},
		{"captcha", 200, `<div id="J_Captcha">`, true},
		{"slidecaptcha", 200, "var slidecaptcha = {}", true},
		{"chinese", 200, "访问被拒绝", true},
		{"access denied", 200, "Access Denied", true},
		{"normal", 200, "<html><body>products</body></html>", false},
	}
	for _, tc := range cases {
		resp := &http.Response{
			StatusCode: tc.status,
			Header:     http.Header{},
		}
		got := isBlocked(resp, []byte(tc.body))
		if got != tc.want {
			t.Errorf("%s: isBlocked = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseSearchJSON(t *testing.T) {
	body := []byte(`<script>
var data = {"offerList": [
  {"offerId": 999, "subject": "Laptop Stand", "priceInfo": {"price": "25.0", "priceTo": "30.0"},
   "tradeQuantity": "50+", "companyName": "Tech Corp", "province": "Fujian"}
]};
</script>`)
	results := parseSearchJSON(body)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].ID != "999" {
		t.Errorf("ID = %q, want 999", results[0].ID)
	}
	if results[0].Title != "Laptop Stand" {
		t.Errorf("Title = %q, want Laptop Stand", results[0].Title)
	}
}

func TestParseProductHTML(t *testing.T) {
	body := []byte(`<html><head></head><body>
<script>
window.__OFFER_DETAIL__ = {
  "subject": "Wireless Mouse",
  "priceRanges": [{"startQuantity": 1, "price": 12.5}, {"startQuantity": 100, "price": 10.0}],
  "saleUnit": "500 pcs",
  "supplier": {"companyName": "Guangzhou Electronics", "province": "Guangdong"},
  "description": "A wireless mouse."
};
</script>
<h1>Wireless Mouse</h1>
</body></html>`)

	p, err := parseProductHTML("12345", body)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Wireless Mouse" {
		t.Errorf("Title = %q, want Wireless Mouse", p.Title)
	}
	if p.Supplier != "Guangzhou Electronics" {
		t.Errorf("Supplier = %q", p.Supplier)
	}
	if p.PriceMin != 10.0 {
		t.Errorf("PriceMin = %v, want 10.0", p.PriceMin)
	}
}
