package infohub_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	ptr "github.com/eclipse-xfsc/microservice-core-go/pkg/ptr"
	goasigner "github.com/eclipse-xfsc/trusted-info-hub/gen/infohub"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/service/infohub"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/service/infohub/infohubfakes"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/storage"
)

func TestNew(t *testing.T) {
	svc := infohub.New(nil, nil, nil, nil, nil, zap.NewNop())
	assert.Implements(t, (*goasigner.Service)(nil), svc)
}

func TestService_Export(t *testing.T) {
	tests := []struct {
		name    string
		req     *goasigner.ExportRequest
		storage *infohubfakes.FakeStorage
		policy  *infohubfakes.FakePolicy
		cache   *infohubfakes.FakeCache
		cred    *infohubfakes.FakeCredentials
		signer  *infohubfakes.FakeSigner

		res     interface{}
		errkind errors.Kind
		errtext string
	}{
		{
			name: "export configuration not found",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return nil, errors.New(errors.NotFound, "export configuration not found")
				},
			},
			errkind: errors.NotFound,
			errtext: "export configuration not found",
		},
		{
			name: "error getting export configuration",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return nil, errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "error getting export data",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return nil, errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "export data not found and triggering export process fails",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
						CacheTTL:   ptr.Int(60),
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return nil, errors.New(errors.NotFound, "no data")
				},
			},
			policy: &infohubfakes.FakePolicy{
				EvaluateStub: func(ctx context.Context, policy string, input interface{}, cachekey string, ttl *int) ([]byte, error) {
					return nil, errors.New("error evaluation policy")
				},
			},
			errkind: errors.Unknown,
			errtext: "error evaluation policy",
		},
		{
			name: "export triggering successfully",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return nil, errors.New(errors.NotFound, "no data")
				},
			},
			policy: &infohubfakes.FakePolicy{
				EvaluateStub: func(ctx context.Context, policy string, input interface{}, cachekey string, ttl *int) ([]byte, error) {
					return []byte(`{"allow":"true"}`), nil
				},
			},
			res: map[string]interface{}{"result": "export request is accepted"},
		},
		{
			name: "export triggering successfully with TTL provided in export configuration",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
						CacheTTL:   ptr.Int(60),
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return nil, errors.New(errors.NotFound, "no data")
				},
			},
			policy: &infohubfakes.FakePolicy{
				EvaluateStub: func(ctx context.Context, policy string, input interface{}, cachekey string, ttl *int) ([]byte, error) {
					return []byte(`{"allow":"true"}`), nil
				},
			},
			res: map[string]interface{}{"result": "export request is accepted"},
		},
		{
			name: "export data is not valid json",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return []byte(`invalid json`), nil
				},
			},
			errkind: errors.Unknown,
			errtext: "error creating export: invalid character",
		},
		{
			name: "error creating verifiable presentation",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return []byte(`{"allow":true}`), nil
				},
			},
			signer: &infohubfakes.FakeSigner{
				CreatePresentationStub: func(ctx context.Context, issuer string, namespace string, key string, data []map[string]interface{}) (map[string]interface{}, error) {
					return nil, errors.New("some error")
				},
			},
			errkind: errors.Unknown,
			errtext: "some error",
		},
		{
			name: "successfully create verifiable presentation",
			req:  &goasigner.ExportRequest{ExportName: "testexport"},
			storage: &infohubfakes.FakeStorage{
				ExportConfigurationStub: func(ctx context.Context, s string) (*storage.ExportConfiguration, error) {
					return &storage.ExportConfiguration{
						ExportName: "testexport",
						Contexts:   []string{"https://www.w3.org/2018/credentials/examples/v1"},
						Policies:   map[string]interface{}{"test/test/1.0": map[string]interface{}{"hello": "test world"}},
					}, nil
				},
			},
			cache: &infohubfakes.FakeCache{
				GetStub: func(ctx context.Context, key string, namespace string, scope string) ([]byte, error) {
					return []byte(`{"allow":true}`), nil
				},
			},
			signer: &infohubfakes.FakeSigner{
				CreatePresentationStub: func(ctx context.Context, issuer string, namespace string, key string, data []map[string]interface{}) (map[string]interface{}, error) {
					return map[string]interface{}{"id": "did:web:example.com"}, nil
				},
			},
			res: map[string]interface{}{"id": "did:web:example.com"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := infohub.New(test.storage, test.policy, test.cache, test.cred, test.signer, zap.NewNop())
			res, err := svc.Export(context.Background(), test.req)
			if err != nil {
				assert.Nil(t, res)
				assert.NotEmpty(t, test.errtext)
				e, ok := err.(*errors.Error)
				assert.True(t, ok)
				assert.Equal(t, test.errkind, e.Kind)
				assert.Contains(t, e.Error(), test.errtext)
			} else {
				assert.Empty(t, test.errtext)
				assert.NotNil(t, res)
				assert.Equal(t, test.res, res)
			}
		})
	}
}
