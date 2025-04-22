package credential

import (
	"net/http"
	"time"

	"github.com/hyperledger/aries-framework-go/pkg/doc/util"
	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"
	"github.com/piprate/json-gold/ld"
)

var defaultContexts = []string{
	"https://www.w3.org/2018/credentials/v1",
	"https://w3id.org/security/suites/jws-2020/v1",
	"https://schema.org",
}

type Credentials struct {
	issuerURI  string
	docLoader  *ld.CachingDocumentLoader
	httpClient *http.Client
}

func New(issuerURI string, httpClient *http.Client) *Credentials {
	loader := ld.NewDefaultDocumentLoader(httpClient)

	return &Credentials{
		issuerURI:  issuerURI,
		docLoader:  ld.NewCachingDocumentLoader(loader),
		httpClient: httpClient,
	}
}

// NewCredential creates a Verifiable Credential without proofs.
func (c *Credentials) NewCredential(contexts []string, subjectID string, subject map[string]interface{}, proof bool) (*verifiable.Credential, error) { //nolint:revive
	jsonldContexts := defaultContexts
	jsonldContexts = append(jsonldContexts, contexts...)

	vc := &verifiable.Credential{
		Context: jsonldContexts,
		Types:   []string{verifiable.VCType},
		Issuer:  verifiable.Issuer{ID: c.issuerURI},
		Issued:  &util.TimeWrapper{Time: time.Now()},
		Subject: verifiable.Subject{
			//ID:           subjectID,
			CustomFields: subject,
		},
	}

	return vc, nil
}

// NewPresentation creates a Verifiable Presentation without proofs.
func (c *Credentials) NewPresentation(contexts []string, vc ...*verifiable.Credential) (*verifiable.Presentation, error) {
	jsonldContexts := defaultContexts
	jsonldContexts = append(jsonldContexts, contexts...)

	vp, err := verifiable.NewPresentation(verifiable.WithCredentials(vc...))
	if err != nil {
		return nil, err
	}
	vp.Context = jsonldContexts
	vp.ID = c.issuerURI
	vp.Type = []string{verifiable.VPType}

	return vp, nil
}

// ParsePresentation without verifying VP proofs.
func (c *Credentials) ParsePresentation(vpBytes []byte) (*verifiable.Presentation, error) {
	return verifiable.ParsePresentation(
		vpBytes,
		verifiable.WithPresDisabledProofCheck(),
		verifiable.WithPresJSONLDDocumentLoader(c.docLoader),
		verifiable.WithPresStrictValidation(),
	)
}
