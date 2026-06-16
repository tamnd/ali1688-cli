package ali1688

import (
	"context"
	"fmt"
	"net/url"
)

// SearchOptions controls a Search call.
type SearchOptions struct {
	MinPrice float64
	MaxPrice float64
	Limit    int
}

// Search searches 1688.com for products matching query.
// Returns ErrBlocked (exit 5) from datacenter IPs.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	base := c.cfg.BaseURL
	if base == "" {
		base = BaseSearchURL
	}
	u := fmt.Sprintf("%s/selloffer/offer_search.htm?keywords=%s&n=y&button_click=right",
		base, url.QueryEscape(query))
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	results, err := parseSearchHTML(body)
	if err != nil {
		return nil, err
	}
	// Apply price filters
	var filtered []SearchResult
	for _, r := range results {
		if opts.MinPrice > 0 && r.PriceMax > 0 && r.PriceMax < opts.MinPrice {
			continue
		}
		if opts.MaxPrice > 0 && r.PriceMin > opts.MaxPrice {
			continue
		}
		filtered = append(filtered, r)
		if opts.Limit > 0 && len(filtered) >= opts.Limit {
			break
		}
	}
	return filtered, nil
}
