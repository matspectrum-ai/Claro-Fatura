package supabase

import (
	"context"
	"net/url"
)

func (c *Client) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var rows []struct {
		Role string `json:"role"`
	}
	q := url.Values{
		"select":  {"role"},
		"user_id": {"eq." + userID},
		"role":    {"eq.admin"},
		"limit":   {"1"},
	}
	if err := c.Select(ctx, "user_roles", q, &rows); err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}
