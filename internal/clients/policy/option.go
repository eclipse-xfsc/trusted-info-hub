package policy

import (
	"net/http"
)

type ClientOption func(*Client)

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}
