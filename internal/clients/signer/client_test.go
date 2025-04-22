package signer_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/clients/signer"
)

func TestClient_CreatePresentation(t *testing.T) {
	tests := []struct {
		name    string
		data    []map[string]interface{}
		handler http.HandlerFunc

		result  map[string]interface{}
		errkind errors.Kind
		errtext string
	}{
		{
			name: "signer returns error",
			data: []map[string]interface{}{{"hello": "world"}},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("some error"))
			},
			errkind: errors.Internal,
			errtext: "some error",
		},
		{
			name: "signer successfully creates verifiable presentation",
			data: []map[string]interface{}{{"hello": "world"}},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"did:web:example.com"}`))
			},
			result: map[string]interface{}{"id": "did:web:example.com"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handler)
			client := signer.New(srv.URL)
			result, err := client.CreatePresentation(context.Background(), "issuer", "namespace", "key", test.data)
			if test.errtext != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.errtext)
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, test.result, result)
			}
		})
	}
}

func TestClient_VerifyPresentation(t *testing.T) {
	tests := []struct {
		name    string
		vp      []byte
		handler http.HandlerFunc
		errkind errors.Kind
		errtext string
	}{
		{
			name: "signer returns error",
			vp:   []byte(`{"id":"did:web:example.com"}`),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("some error"))
			},
			errkind: errors.Internal,
			errtext: "some error",
		},
		{
			name: "signer returns unexpected response",
			vp:   []byte(`{"id":"did:web:example.com"}`),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("invalid json"))
			},
			errkind: errors.Unknown,
			errtext: "failed to decode response",
		},
		{
			name: "signer returns successfully",
			vp:   []byte(`{"id":"did:web:example.com"}`),
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"did:web:example.com"}`))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handler)
			client := signer.New(srv.URL)
			err := client.VerifyPresentation(context.Background(), test.vp)
			if err != nil {
				assert.Contains(t, err.Error(), test.errtext)
			} else {
				assert.Empty(t, test.errtext)
			}
		})
	}
}
