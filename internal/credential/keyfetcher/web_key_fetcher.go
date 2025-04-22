package keyfetcher

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-jose/go-jose/v3"
	ariesjwk "github.com/hyperledger/aries-framework-go/pkg/doc/jose/jwk"
	"github.com/hyperledger/aries-framework-go/pkg/doc/signature/verifier"
	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

type VerificationMethod struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	PublicKeyJWK *jose.JSONWebKey `json:"publicKeyJWK"`
}

type WebKeyFetcher struct {
	httpClient *http.Client
}

// NewWebKeyFetcher retrieves a public key by directly calling an HTTP URL.
func NewWebKeyFetcher(httpClient *http.Client) verifiable.PublicKeyFetcher {
	f := &WebKeyFetcher{httpClient: httpClient}
	return f.fetch
}

// fetch a public key directly from an HTTP URL. issuerID is expected to be
// URL like https://example.com/keys and keyID is the name of the key to be fetched.
// If the keyID contains a fragment(#), it is removed when constructing the target URL.
func (f *WebKeyFetcher) fetch(issuerID, keyID string) (*verifier.PublicKey, error) {
	// If keyID is prefixed with hashtag(#) it must be removed
	keyID = strings.TrimPrefix(keyID, "#")

	// Construct URL like http://signer:8080/v1/keys/key-1
	addr := fmt.Sprintf("%s/%s", issuerID, keyID)
	uri, err := url.ParseRequestURI(addr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, errors.New(errors.NotFound, "key not found")
		}
		return nil, errors.New(errors.GetKind(resp.StatusCode), fmt.Errorf("unexpected response: %s", resp.Status))
	}

	var verificationMethod VerificationMethod
	if err := json.NewDecoder(resp.Body).Decode(&verificationMethod); err != nil {
		return nil, err
	}

	if verificationMethod.PublicKeyJWK == nil {
		return nil, fmt.Errorf("public key not found after decoding response")
	}

	// We need to extract the Curve and Kty values as they are needed by the
	// Aries public key verifiers.
	curve, kty, err := keyParams(verificationMethod.PublicKeyJWK.Key)
	if err != nil {
		return nil, err
	}

	return &verifier.PublicKey{
		Type: "JsonWebKey2020",
		JWK: &ariesjwk.JWK{
			JSONWebKey: *verificationMethod.PublicKeyJWK,
			Crv:        curve,
			Kty:        kty,
		},
	}, nil
}

func keyParams(key interface{}) (curve string, kty string, err error) {
	switch k := key.(type) {
	case *ecdsa.PublicKey:
		return k.Curve.Params().Name, "EC", nil
	case *ed25519.PublicKey:
		return "ED25519", "OKP", nil
	case *rsa.PublicKey:
		return "", "RSA", nil
	default:
		return "", "", fmt.Errorf("unknown key type: %T", k)
	}
}
