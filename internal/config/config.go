package config

import "time"

type Config struct {
	HTTP       httpConfig
	Mongo      mongoConfig
	Policy     policyConfig
	Cache      cacheConfig
	Credential credentialConfig
	Signer     signerConfig
	Metrics    metricsConfig
	OAuth      oauthConfig
	Auth       authConfig

	LogLevel string `envconfig:"LOG_LEVEL" default:"INFO"`
}

type httpConfig struct {
	Host         string        `envconfig:"HTTP_HOST"`
	Port         string        `envconfig:"HTTP_PORT" default:"8080"`
	IdleTimeout  time.Duration `envconfig:"HTTP_IDLE_TIMEOUT" default:"120s"`
	ReadTimeout  time.Duration `envconfig:"HTTP_READ_TIMEOUT" default:"10s"`
	WriteTimeout time.Duration `envconfig:"HTTP_WRITE_TIMEOUT" default:"10s"`
}

type mongoConfig struct {
	Addr          string `envconfig:"MONGO_ADDR" required:"true"`
	User          string `envconfig:"MONGO_USER" required:"true"`
	Pass          string `envconfig:"MONGO_PASS" required:"true"`
	DB            string `envconfig:"MONGO_DBNAME" default:"infohub"`
	Collection    string `envconfig:"MONGO_COLLECTION" default:"exports"`
	AuthMechanism string `envconfig:"MONGO_AUTH_MECHANISM" default:"SCRAM-SHA-1"`
}

type credentialConfig struct {
	IssuerURI string `envconfig:"ISSUER_URI" required:"true"`
}

type policyConfig struct {
	Addr string `envconfig:"POLICY_ADDR" required:"true"`
}

type cacheConfig struct {
	Addr string `envconfig:"CACHE_ADDR" required:"true"`
}

type signerConfig struct {
	Addr string `envconfig:"SIGNER_ADDR" required:"true"`
}

type metricsConfig struct {
	Addr string `envconfig:"METRICS_ADDR" default:":2112"`
}

type oauthConfig struct {
	ClientID     string `envconfig:"OAUTH_CLIENT_ID"`
	ClientSecret string `envconfig:"OAUTH_CLIENT_SECRET"`
	TokenURL     string `envconfig:"OAUTH_TOKEN_URL"`
}

type authConfig struct {
	Enabled         bool          `envconfig:"AUTH_ENABLED" default:"false"`
	JwkURL          string        `envconfig:"AUTH_JWK_URL"`
	RefreshInterval time.Duration `envconfig:"AUTH_REFRESH_INTERVAL" default:"1h"`
}
