package ali1688

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// --- Search HTML parsing ---

// parseSearchHTML extracts product listings from the 1688 search result page.
// It first tries to find a JSON block in a script tag; if that fails it falls
// back to a simple HTML regex extraction.
func parseSearchHTML(body []byte) ([]SearchResult, error) {
	// Strategy A: look for JSON embedded in script tags
	if results := parseSearchJSON(body); len(results) > 0 {
		return results, nil
	}
	// Strategy B: HTML card extraction
	return parseSearchCards(body), nil
}

var (
	// offerListRE matches the offerList JSON array in script blocks.
	offerListRE = regexp.MustCompile(`(?s)"offerList"\s*:\s*(\[.+?\])`)

	// initDataRE matches window.__INIT_DATA__ = {...}
	initDataRE = regexp.MustCompile(`(?s)window\.__INIT_DATA__\s*=\s*(\{.+?\})\s*;`)
)

type rawOffer struct {
	OfferID   interface{} `json:"offerId"`
	Subject   string      `json:"subject"`
	PriceInfo struct {
		Price   string `json:"price"`
		PriceTO string `json:"priceTo"`
	} `json:"priceInfo"`
	TradeQuantity string `json:"tradeQuantity"`
	CompanyName   string `json:"companyName"`
	Province      string `json:"province"`
	OfferDetailURL string `json:"detailPageUrl"`
}

func parseSearchJSON(body []byte) []SearchResult {
	s := string(body)
	m := offerListRE.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	var offers []rawOffer
	if err := json.Unmarshal([]byte(m[1]), &offers); err != nil {
		return nil
	}
	return rawOffersToResults(offers)
}

func rawOffersToResults(offers []rawOffer) []SearchResult {
	out := make([]SearchResult, 0, len(offers))
	for _, o := range offers {
		id := fmt.Sprintf("%v", o.OfferID)
		pMin, _ := strconv.ParseFloat(o.PriceInfo.Price, 64)
		pMax, _ := strconv.ParseFloat(o.PriceInfo.PriceTO, 64)
		if pMax == 0 {
			pMax = pMin
		}
		u := o.OfferDetailURL
		if u == "" && id != "" {
			u = BaseDetailURL + "/offer/" + id + ".html"
		}
		out = append(out, SearchResult{
			ID:       id,
			Title:    o.Subject,
			PriceMin: pMin,
			PriceMax: pMax,
			Currency: "CNY",
			MOQ:      o.TradeQuantity,
			Supplier: o.CompanyName,
			Location: o.Province,
			URL:      u,
		})
	}
	return out
}

// HTML card extraction regexps (fallback when no JSON is embedded).
var (
	offerCardRE  = regexp.MustCompile(`(?s)class="[^"]*offer-item[^"]*"[^>]*>(.*?)</(?:li|div)[^>]*>`)
	titleRE      = regexp.MustCompile(`(?:class="[^"]*offer-title[^"]*"[^>]*>|class="[^"]*title[^"]*"[^>]*>)\s*<[^>]+>([^<]+)</`)
	priceRE      = regexp.MustCompile(`class="[^"]*price[^"]*b-price[^"]*"[^>]*>([^<]+)<`)
	moqRE        = regexp.MustCompile(`class="[^"]*moq[^"]*"[^>]*>([^<]+)<`)
	supplierRE   = regexp.MustCompile(`class="[^"]*company-name[^"]*"[^>]*>([^<]+)<`)
	locationRE   = regexp.MustCompile(`class="[^"]*location[^"]*"[^>]*>([^<]+)<`)
	offerLinkRE  = regexp.MustCompile(`href="(https?://[^"]*detail\.1688\.com/offer/(\d+)\.html)"`)
	priceRangeRE = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[-–]\s*(\d+(?:\.\d+)?)`)
	singlePriceRE = regexp.MustCompile(`(\d+(?:\.\d+)?)`)
)

func parseSearchCards(body []byte) []SearchResult {
	s := string(body)
	var out []SearchResult
	for _, card := range offerCardRE.FindAllString(s, -1) {
		r := extractCardResult(card)
		if r.ID != "" || r.Title != "" {
			out = append(out, r)
		}
	}
	return out
}

func extractCardResult(card string) SearchResult {
	var r SearchResult
	r.Currency = "CNY"

	if m := offerLinkRE.FindStringSubmatch(card); m != nil {
		r.URL = m[1]
		r.ID = m[2]
	}
	if m := titleRE.FindStringSubmatch(card); m != nil {
		r.Title = strings.TrimSpace(stripTags(m[1]))
	}
	if m := priceRE.FindStringSubmatch(card); m != nil {
		ps := strings.TrimSpace(m[1])
		if rm := priceRangeRE.FindStringSubmatch(ps); rm != nil {
			r.PriceMin, _ = strconv.ParseFloat(rm[1], 64)
			r.PriceMax, _ = strconv.ParseFloat(rm[2], 64)
		} else if rm2 := singlePriceRE.FindStringSubmatch(ps); rm2 != nil {
			r.PriceMin, _ = strconv.ParseFloat(rm2[1], 64)
			r.PriceMax = r.PriceMin
		}
	}
	if m := moqRE.FindStringSubmatch(card); m != nil {
		r.MOQ = strings.TrimSpace(m[1])
	}
	if m := supplierRE.FindStringSubmatch(card); m != nil {
		r.Supplier = strings.TrimSpace(stripTags(m[1]))
	}
	if m := locationRE.FindStringSubmatch(card); m != nil {
		r.Location = strings.TrimSpace(m[1])
	}
	return r
}

// --- Product HTML parsing ---

type rawProductDetail struct {
	Title    string `json:"subject"`
	PriceRanges []struct {
		StartQuantity int     `json:"startQuantity"`
		Price         float64 `json:"price"`
	} `json:"priceRanges"`
	TradeQuantity string `json:"saleUnit"`
	Supplier      struct {
		CompanyName string `json:"companyName"`
		Province    string `json:"province"`
	} `json:"supplier"`
	Description string `json:"description"`
}

var (
	offerDetailRE = regexp.MustCompile(`(?s)window\.__OFFER_DETAIL__\s*=\s*(\{.+?\})\s*;`)
	initDataRE2   = regexp.MustCompile(`(?s)window\.__initData__\s*=\s*(\{.+?\})\s*;`)
	h1RE          = regexp.MustCompile(`<h1[^>]*>([^<]+)</h1>`)
)

func parseProductHTML(id string, body []byte) (*Product, error) {
	s := string(body)

	// Try to find JSON in script tags
	for _, re := range []*regexp.Regexp{offerDetailRE, initDataRE2} {
		if m := re.FindStringSubmatch(s); m != nil {
			p, err := parseProductJSON(id, []byte(m[1]))
			if err == nil {
				return p, nil
			}
		}
	}

	// HTML fallback
	p := &Product{
		ID:       id,
		Currency: "CNY",
		URL:      BaseDetailURL + "/offer/" + id + ".html",
	}
	if m := h1RE.FindStringSubmatch(s); m != nil {
		p.Title = strings.TrimSpace(stripTags(m[1]))
	}
	return p, nil
}

func parseProductJSON(id string, data []byte) (*Product, error) {
	var raw rawProductDetail
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse product JSON: %w", err)
	}
	p := &Product{
		ID:          id,
		Title:       raw.Title,
		MOQ:         raw.TradeQuantity,
		Supplier:    raw.Supplier.CompanyName,
		Location:    raw.Supplier.Province,
		Description: raw.Description,
		Currency:    "CNY",
		URL:         BaseDetailURL + "/offer/" + id + ".html",
	}
	for _, pr := range raw.PriceRanges {
		p.PriceRanges = append(p.PriceRanges,
			fmt.Sprintf(">=%d: %.2f CNY", pr.StartQuantity, pr.Price))
		if p.PriceMin == 0 || pr.Price < p.PriceMin {
			p.PriceMin = pr.Price
		}
		if pr.Price > p.PriceMax {
			p.PriceMax = pr.Price
		}
	}
	return p, nil
}

// stripTags removes HTML tags from a string.
var tagRE2 = regexp.MustCompile(`<[^>]+>`)

func stripTags(s string) string {
	return tagRE2.ReplaceAllString(s, "")
}
