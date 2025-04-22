package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

const (
	headerEvaluationID = "x-evaluation-id"
	headerCacheTTL     = "x-cache-ttl"
)

type Client struct {
	addr       string
	httpClient *http.Client
}

func New(addr string, opts ...ClientOption) *Client {
	c := &Client{
		addr:       addr,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Evaluate calls the policy service to execute the given policy.
// The policy is expected as a string path uniquely identifying the
// policy that has to be evaluated. For example, with policy = `xfsc/didResolve/1.0`,
// the client will do HTTP request to http://policyhost/policy/xfsc/didResolve/1.0/evaluation.
func (c *Client) Evaluate(ctx context.Context, policy string, data interface{}, evaluationID string, cacheTTL *int) ([]byte, error) {
	uri := c.addr + "/policy/" + policy + "/evaluation"
	policyURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, errors.New(errors.BadRequest, "invalid policy evaluation URL", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, policyURL.String(), bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	if evaluationID != "" {
		req.Header.Set(headerEvaluationID, evaluationID)
	}

	if cacheTTL != nil {
		req.Header.Set(headerCacheTTL, strconv.Itoa(*cacheTTL))
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response on policy evaluation: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
