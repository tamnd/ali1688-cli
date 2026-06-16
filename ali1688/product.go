package ali1688

import (
	"context"
	"fmt"
)

// GetProduct fetches a single 1688.com offer by its numeric ID.
// Returns ErrBlocked (exit 5) from datacenter IPs.
func (c *Client) GetProduct(ctx context.Context, id string) (*Product, error) {
	u := fmt.Sprintf("%s/offer/%s.html", BaseDetailURL, id)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseProductHTML(id, body)
}
