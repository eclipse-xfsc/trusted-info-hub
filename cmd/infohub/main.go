package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	goahttp "goa.design/goa/v3/http"
	goa "goa.design/goa/v3/pkg"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/sync/errgroup"

	auth "github.com/eclipse-xfsc/microservice-core-go/pkg/auth"
	cache "github.com/eclipse-xfsc/microservice-core-go/pkg/cache"
	goadec "github.com/eclipse-xfsc/microservice-core-go/pkg/goadec"
	graceful "github.com/eclipse-xfsc/microservice-core-go/pkg/graceful"
	goahealth "github.com/eclipse-xfsc/trusted-info-hub/gen/health"
	goahealthsrv "github.com/eclipse-xfsc/trusted-info-hub/gen/http/health/server"
	goainfohubsrv "github.com/eclipse-xfsc/trusted-info-hub/gen/http/infohub/server"
	goaopenapisrv "github.com/eclipse-xfsc/trusted-info-hub/gen/http/openapi/server"
	goainfohub "github.com/eclipse-xfsc/trusted-info-hub/gen/infohub"
	"github.com/eclipse-xfsc/trusted-info-hub/gen/openapi"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/clients/policy"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/clients/signer"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/config"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/credential"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/service"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/service/health"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/service/infohub"
	"github.com/eclipse-xfsc/trusted-info-hub/internal/storage"
)

var Version = "0.0.0+development"

func main() {
	// load configuration from environment
	var cfg config.Config
	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatalf("cannot load configuration: %v\n", err)
	}

	// create logger
	logger, err := createLogger(cfg.LogLevel)
	if err != nil {
		log.Fatalln(err)
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("infohub service started", zap.String("version", Version), zap.String("goa", goa.Version()))

	// connect to mongo db
	db, err := mongo.Connect(
		context.Background(),
		options.Client().ApplyURI(cfg.Mongo.Addr).SetAuth(options.Credential{
			AuthMechanism: cfg.Mongo.AuthMechanism,
			Username:      cfg.Mongo.User,
			Password:      cfg.Mongo.Pass,
		}),
	)
	if err != nil {
		logger.Fatal("error connecting to mongodb", zap.Error(err))
	}
	defer db.Disconnect(context.Background()) //nolint:errcheck

	// create storage
	storage, err := storage.New(db, cfg.Mongo.DB, cfg.Mongo.Collection, logger)
	if err != nil {
		logger.Fatal("error connecting to database", zap.Error(err))
	}

	httpClient := httpClient()

	oauthClient := httpClient
	if cfg.Auth.Enabled {
		// Create an HTTP Client which automatically issues and carries an OAuth2 token.
		// The token will auto-refresh when its expiration is near.
		oauthCtx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
		oauthClient = newOAuth2Client(oauthCtx, cfg.OAuth.ClientID, cfg.OAuth.ClientSecret, cfg.OAuth.TokenURL)
	}

	credentials := credential.New(cfg.Credential.IssuerURI, httpClient)

	// create policy client
	policy := policy.New(cfg.Policy.Addr, policy.WithHTTPClient(oauthClient))

	// create cache client
	cache := cache.New(cfg.Cache.Addr, cache.WithHTTPClient(oauthClient))

	// create signer client
	signer := signer.New(cfg.Signer.Addr, signer.WithHTTPClient(oauthClient))

	// create services
	var (
		infohubSvc goainfohub.Service
		healthSvc  goahealth.Service
	)
	{
		infohubSvc = infohub.New(storage, policy, cache, credentials, signer, logger)
		healthSvc = health.New(Version)
	}

	// create endpoints
	var (
		infohubEndpoints *goainfohub.Endpoints
		healthEndpoints  *goahealth.Endpoints
		openapiEndpoints *openapi.Endpoints
	)
	{
		infohubEndpoints = goainfohub.NewEndpoints(infohubSvc)
		healthEndpoints = goahealth.NewEndpoints(healthSvc)
		openapiEndpoints = openapi.NewEndpoints(nil)
	}

	// Provide the transport specific request decoder and response encoder.
	// The goa http package has built-in support for JSON, XML and gob.
	// Other encodings can be used by providing the corresponding functions,
	// see goa.design/implement/encoding.
	var (
		dec = goahttp.RequestDecoder
		enc = goahttp.ResponseEncoder
	)

	// Build the service HTTP request multiplexer and configure it to serve
	// HTTP requests to the service endpoints.
	mux := goahttp.NewMuxer()

	// Wrap the endpoints with the transport specific layers. The generated
	// server packages contains code generated from the design which maps
	// the service input and output data structures to HTTP requests and
	// responses.
	var (
		infohubServer *goainfohubsrv.Server
		healthServer  *goahealthsrv.Server
		openapiServer *goaopenapisrv.Server
	)
	{
		infohubServer = goainfohubsrv.New(infohubEndpoints, mux, dec, enc, nil, errFormatter)
		healthServer = goahealthsrv.New(healthEndpoints, mux, dec, enc, nil, errFormatter)
		openapiServer = goaopenapisrv.New(openapiEndpoints, mux, dec, enc, nil, errFormatter, nil, nil)
	}

	// set custom request decoder, so that request body bytes are simply
	// read and not decoded in some other way
	infohubServer.Import = goainfohubsrv.NewImportHandler(
		infohubEndpoints.Import,
		mux,
		goadec.BytesDecoder,
		enc,
		nil,
		errFormatter,
	)

	// Apply Authentication middleware if enabled
	if cfg.Auth.Enabled {
		m, err := auth.NewMiddleware(cfg.Auth.JwkURL, cfg.Auth.RefreshInterval, httpClient)
		if err != nil {
			logger.Fatal("failed to create authentication middleware", zap.Error(err))
		}
		infohubServer.Use(m.Handler())
	}

	// Configure the mux.
	goainfohubsrv.Mount(mux, infohubServer)
	goahealthsrv.Mount(mux, healthServer)
	goaopenapisrv.Mount(mux, openapiServer)

	// expose metrics
	go exposeMetrics(cfg.Metrics.Addr, logger)

	var handler http.Handler = mux
	srv := &http.Server{
		Addr:         cfg.HTTP.Host + ":" + cfg.HTTP.Port,
		Handler:      handler,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	g, ctx := errgroup.WithContext(context.Background())
	g.Go(func() error {
		if err := graceful.Shutdown(ctx, srv, 20*time.Second); err != nil {
			logger.Error("server shutdown error", zap.Error(err))
			return err
		}
		return errors.New("server stopped successfully")
	})
	if err := g.Wait(); err != nil {
		logger.Error("run group stopped", zap.Error(err))
	}

	logger.Info("bye bye")
}

func createLogger(logLevel string, opts ...zap.Option) (*zap.Logger, error) {
	var level = zapcore.InfoLevel
	if logLevel != "" {
		err := level.UnmarshalText([]byte(logLevel))
		if err != nil {
			return nil, err
		}
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(level)
	config.DisableStacktrace = true
	config.EncoderConfig.TimeKey = "ts"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	return config.Build(opts...)
}

func errFormatter(ctx context.Context, e error) goahttp.Statuser {
	return service.NewErrorResponse(ctx, e)
}

func httpClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     60 * time.Second,
		},
		Timeout: 10 * time.Second,
	}
}

func newOAuth2Client(ctx context.Context, cID, cSecret, tokenURL string) *http.Client {
	oauthCfg := clientcredentials.Config{
		ClientID:     cID,
		ClientSecret: cSecret,
		TokenURL:     tokenURL,
	}

	return oauthCfg.Client(ctx)
}

func exposeMetrics(addr string, logger *zap.Logger) {
	promMux := http.NewServeMux()
	promMux.Handle("/metrics", promhttp.Handler())
	logger.Info(fmt.Sprintf("exposing prometheus metrics at %s/metrics", addr))
	if err := http.ListenAndServe(addr, promMux); err != nil { //nolint:gosec
		logger.Error("error exposing prometheus metrics", zap.Error(err))
	}
}
