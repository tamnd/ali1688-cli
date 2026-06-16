package ali1688

// SearchResult is one item from a 1688.com product search.
type SearchResult struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	PriceMin float64 `json:"price_min"`
	PriceMax float64 `json:"price_max"`
	Currency string  `json:"currency"`
	MOQ      string  `json:"moq"`
	Supplier string  `json:"supplier"`
	Location string  `json:"location"`
	URL      string  `json:"url"`
}

// Product is the full record for a single 1688.com offer.
type Product struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	PriceRanges []string `json:"price_ranges"`
	PriceMin    float64  `json:"price_min"`
	PriceMax    float64  `json:"price_max"`
	Currency    string   `json:"currency"`
	MOQ         string   `json:"moq"`
	Supplier    string   `json:"supplier"`
	Location    string   `json:"location"`
	URL         string   `json:"url"`
	Description string   `json:"description,omitempty"`
}
