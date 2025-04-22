package infohub

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hyperledger/aries-framework-go/pkg/doc/verifiable"
	"go.uber.org/zap"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
	"github.com/eclipse-xfsc/trusted-info-hub/gen/infohub"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/storage"
)

//go:generate counterfeiter . Storage
//go:generate counterfeiter . Policy
//go:generate counterfeiter . Cache
//go:generate counterfeiter . Credentials
//go:generate counterfeiter . Signer

var exportAccepted = map[string]interface{}{"result": "export request is accepted"}

type Storage interface {
	ExportConfiguration(ctx context.Context, exportName string) (*storage.ExportConfiguration, error)
}

type Policy interface {
	Evaluate(ctx context.Context, policy string, data interface{}, evaluationID string, ttl *int) ([]byte, error)
}

type Cache interface {
	Get(ctx context.Context, key, namespace, scope string) ([]byte, error)
	Set(ctx context.Context, key, namespace, scope string, value []byte) error
}

type Credentials interface {
	ParsePresentation(vpBytes []byte) (*verifiable.Presentation, error)
}

type Signer interface {
	CreatePresentation(ctx context.Context, issuer, namespace, key string, data []map[string]interface{}) (map[string]interface{}, error)
	VerifyPresentation(ctx context.Context, vp []byte) error
}

type Service struct {
	storage     Storage
	policy      Policy
	cache       Cache
	credentials Credentials
	signer      Signer
	logger      *zap.Logger
}

func New(storage Storage, policy Policy, cache Cache, cred Credentials, signer Signer, logger *zap.Logger) *Service {
	return &Service{
		storage:     storage,
		policy:      policy,
		cache:       cache,
		credentials: cred,
		signer:      signer,
		logger:      logger,
	}
}

// Import the given data wrapped as Verifiable Presentation into the Cache.
func (s *Service) Import(ctx context.Context, req *infohub.ImportRequest) (res *infohub.ImportResult, err error) {
	logger := s.logger.With(zap.String("operation", "import"))

	if err := s.signer.VerifyPresentation(ctx, req.Data); err != nil {
		logger.Error("error verifying presentation", zap.Error(err))
		return nil, err
	}

	vp, err := s.credentials.ParsePresentation(req.Data)
	if err != nil {
		logger.Error("error parsing verifiable presentation", zap.Error(err))
		return nil, err
	}

	// separate data entries are wrapped in separate verifiable credentials;
	// each one of them must be placed separately in the cache
	var importedCredentials []string
	for _, credential := range vp.Credentials() {
		cred, ok := credential.(map[string]interface{})
		if !ok {
			logger.Warn("verifiable presentation contains unknown credential type")
			return nil, errors.New(errors.BadRequest, "verifiable presentation contains unknown credential type")
		}

		if cred["credentialSubject"] == nil {
			logger.Error("verifiable credential doesn't contain subject")
			return nil, errors.New(errors.BadRequest, "verifiable credential doesn't contain subject")
		}

		subject, ok := cred["credentialSubject"].(map[string]interface{})
		if !ok {
			logger.Error("verifiable credential subject is not a map object")
			return nil, errors.New(errors.BadRequest, "verifiable credential subject is not a map object")
		}

		subjectBytes, err := json.Marshal(subject)
		if err != nil {
			logger.Error("error encoding subject to json", zap.Error(err))
			return nil, errors.New("error encoding subject to json")
		}

		importID := uuid.NewString()
		if err := s.cache.Set(ctx, importID, "", "", subjectBytes); err != nil {
			logger.Error("error saving imported data to cache", zap.Error(err))
			continue
		}
		importedCredentials = append(importedCredentials, importID)
	}

	return &infohub.ImportResult{ImportIds: importedCredentials}, nil
}

func (s *Service) Export(ctx context.Context, req *infohub.ExportRequest) (interface{}, error) {
	logger := s.logger.With(
		zap.String("operation", "export"),
		zap.String("exportName", req.ExportName),
	)

	exportCfg, err := s.storage.ExportConfiguration(ctx, req.ExportName)
	if err != nil {
		logger.Error("error getting export configuration", zap.Error(err))
		return nil, err
	}

	// get policy names needed for the export
	var policyNames []string
	for name := range exportCfg.Policies {
		policyNames = append(policyNames, name)
	}

	// get the results of all policies configured in the export
	policyResults, err := s.getExportData(ctx, exportCfg.ExportName, policyNames)
	if err != nil {
		if errors.Is(errors.NotFound, err) {
			if err := s.triggerExport(ctx, exportCfg); err != nil {
				logger.Error("error performing export", zap.Error(err))
				return nil, err
			}
			return exportAccepted, nil
		}
		logger.Error("failed to get policy results from cache", zap.Error(err))
		return nil, err
	}

	var results []map[string]interface{}
	for policy, result := range policyResults {
		var res map[string]interface{}
		if err := json.Unmarshal(result, &res); err != nil {
			logger.Error("error decoding policy result as json", zap.Error(err), zap.String("policy", policy))
			return nil, errors.New("error creating export", err)
		}
		results = append(results, res)
	}

	// create verifiable presentation
	vp, err := s.signer.CreatePresentation(
		ctx,
		exportCfg.Issuer,
		exportCfg.KeyNamespace,
		exportCfg.Key,
		results,
	)
	if err != nil {
		logger.Error("error creating verifiable presentation", zap.Error(err))
		return nil, errors.New("error creating export", err)
	}

	return vp, nil
}

// getExportData retrieves from Cache the serialized policy execution results.
// If result for a given policy name is not found in the Cache, a NotFound error
// is returned.
// If all results are found, they are returned as map, where the key is policyName
// and the value is the JSON serialized bytes of the policy result.
//
// policyNames are formatted as 'group/policy/version' string, e.g. 'example/example/1.0'
func (s *Service) getExportData(ctx context.Context, exportName string, policyNames []string) (map[string][]byte, error) {
	results := make(map[string][]byte)
	for _, policy := range policyNames {
		res, err := s.cache.Get(ctx, exportCacheKey(exportName, policy), "", "")
		if err != nil {
			return nil, err
		}
		results[policy] = res
	}

	return results, nil
}

func (s *Service) triggerExport(ctx context.Context, exportCfg *storage.ExportConfiguration) error {
	s.logger.Info("export triggered", zap.String("exportName", exportCfg.ExportName))
	for policy, input := range exportCfg.Policies {
		cacheKey := exportCacheKey(exportCfg.ExportName, policy)
		_, err := s.policy.Evaluate(ctx, policy, input, cacheKey, exportCfg.CacheTTL)
		if err != nil {
			return err
		}
	}
	return nil
}

func exportCacheKey(exportName string, policyName string) string {
	return exportName + ":" + policyName
}
