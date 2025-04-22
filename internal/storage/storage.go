package storage

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	errors "github.com/eclipse-xfsc/microservice-core-go/pkg/err"
)

type ExportConfiguration struct {
	ExportName   string
	Contexts     []string
	Policies     map[string]interface{}
	CacheTTL     *int
	Issuer       string // issuer DID
	KeyNamespace string // signing key namespace
	Key          string // signing key name
}

type Storage struct {
	exportConfig *mongo.Collection
	logger       *zap.Logger
}

func New(db *mongo.Client, dbname, collection string, logger *zap.Logger) (*Storage, error) {
	if err := db.Ping(context.Background(), nil); err != nil {
		return nil, err
	}
	return &Storage{
		exportConfig: db.Database(dbname).Collection(collection),
		logger:       logger,
	}, nil
}

func (s *Storage) ExportConfiguration(ctx context.Context, exportName string) (*ExportConfiguration, error) {
	result := s.exportConfig.FindOne(ctx, bson.M{
		"exportName": exportName,
	}, options.FindOne().SetCollation(&options.Collation{
		Locale:   "en",
		Strength: 2,
	}))

	if result.Err() != nil {
		if strings.Contains(result.Err().Error(), "no documents in result") {
			return nil, errors.New(errors.NotFound, "export configuration not found")
		}
		return nil, result.Err()
	}

	var expcfg ExportConfiguration
	if err := result.Decode(&expcfg); err != nil {
		return nil, err
	}

	return &expcfg, nil
}
