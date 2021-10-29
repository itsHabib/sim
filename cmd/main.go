package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/caarlos0/env/v6"
	"github.com/couchbase/gocb/v2"
	"go.uber.org/zap"

	"github.com/itsHabib/sim/internal/images"
	"github.com/itsHabib/sim/internal/images/reader"
	"github.com/itsHabib/sim/internal/images/service"
	"github.com/itsHabib/sim/internal/images/writer"
	"github.com/itsHabib/sim/internal/runner"
)

type config struct {
	Debug bool `env:"DEBUG" envDefault:"false"`

	LocalstackURL string `env:"LOCALSTACK_URL"`

	Region string `env:"REGION,required"`

	Storage string `env:"STORAGE,required"`

	CouchbaseEndpoint string `env:"COUCHBASE_ENDPOINT,required"`
	CouchbaseUsername string `env:"COUCHBASE_USERNAME,required"`
	CouchbasePassword string `env:"COUCHBASE_PASSWORD,required"`
	CouchbaseBucket   string `env:"COUCHBASE_BUCKET,required"`
}

func main() {
	cfg, err := initConfig()
	if err != nil {
		log.Fatalf("unable to get config: %s", err)
	}

	logger, err := getLogger(cfg.Debug)
	if err != nil {
		log.Fatalf("unable to get logger: %s", err)
	}

	cluster, err := getCluster(cfg)
	if err != nil {
		log.Fatalf("unable to get cb cluster connection: %s", err)
	}

	writer, err := writer.NewService(logger, cluster, cfg.CouchbaseBucket)
	if err != nil {
		log.Fatalf("unable to get writer: %s", err)
	}
	reader, err := reader.NewService(logger, cluster, cfg.CouchbaseBucket)
	if err != nil {
		log.Fatalf("unable to get reader: %s", err)
	}

	awsCfg := getCfg(cfg)
	svc, err := service.New(logger, cfg.Storage, reader, writer, images.WithSessionOptions(awsCfg))
	if err != nil {
		log.Fatalf("unable to get service: %s", err)
	}
	runner := runner.NewRunner(logger, svc)

	if err := runner.Run(); err != nil {
		log.Fatalf("run err: %s", err)
	}
}

func getCfg(cfg *config) *aws.Config {
	config := aws.
		NewConfig().
		WithRegion(cfg.Region)

	if cfg.LocalstackURL != "" {
		config = config.
			WithEndpoint(cfg.LocalstackURL).
			WithS3ForcePathStyle(true).
			WithCredentials(credentials.NewStaticCredentials("images", "secret", ""))
	}

	return config
}

func getCluster(cfg *config) (*gocb.Cluster, error) {
	return gocb.Connect(
		cfg.CouchbaseEndpoint,
		gocb.ClusterOptions{
			Username: cfg.CouchbaseUsername,
			Password: cfg.CouchbasePassword,
		},
	)
}

func getLogger(debug bool) (*zap.Logger, error) {
	if !debug {
		return zap.NewNop(), nil
	}

	logger, err := zap.NewDevelopment(zap.WithCaller(true))
	if err != nil {
		return nil, err
	}

	return logger, nil
}

func initConfig() (*config, error) {
	cfg := new(config)
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
