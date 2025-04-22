package signer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/piprate/json-gold/ld"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

const (
	createPresentationPath = "/v1/presentation"
	presentationVerifyPath = "/v1/presentation/verify"
)

type Client struct {
	addr       string
	httpClient *http.Client
	docLoader  *ld.CachingDocumentLoader
}

func New(addr string, opts ...ClientOption) *Client {
	c := &Client{
		addr:       addr,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.docLoader = ld.NewCachingDocumentLoader(ld.NewDefaultDocumentLoader(c.httpClient))

	return c
}

func (c *Client) CreatePresentation(ctx context.Context, issuer, namespace, key string, data []map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"issuer":    issuer,
		"namespace": namespace,
		"key":       key,
		"data":      data,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.addr+createPresentationPath, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(errors.GetKind(resp.StatusCode), getErrorBody(resp))
	}

	var presentation map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&presentation); err != nil {
		return nil, errors.New("error decoding signer response as verifiable presentation", err)
	}

	return presentation, nil
}

func (c *Client) VerifyPresentation(ctx context.Context, vp []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", c.addr+presentationVerifyPath, bytes.NewReader(vp))
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errors.GetKind(resp.StatusCode), getErrorBody(resp))
	}

	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return errors.New("failed to decode response", err)
	}

	if !result.Valid {
		return errors.New("invalid presentation proof")
	}

	return nil
}

func getErrorBody(resp *http.Response) string {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return string(body)
}
