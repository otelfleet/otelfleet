package main

import (
	"context"
	"crypto"
	"crypto/tls"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"database/sql"

	"github.com/cockroachdb/pebble/v2"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/otelfleet/otelfleet/pkg/api/bootstrap/v1alpha1"
	_ "github.com/otelfleet/otelfleet/pkg/logger"
	"github.com/otelfleet/otelfleet/pkg/server"
	otelpebble "github.com/otelfleet/otelfleet/pkg/storage/pebble"
)

var (
	servingCertDataFile = "./localhost.pem"
	servingKeyDataFile  = "./localhost-key.pem"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func loadCerts() *tls.Certificate {
	servingCertData, err := os.ReadFile(servingCertDataFile)
	if err != nil {
		panic(err)
	}
	servingKeyData, err := os.ReadFile(servingKeyDataFile)
	if err != nil {
		panic(err)
	}
	servingCert, err := tls.X509KeyPair(servingCertData, servingKeyData)
	if err != nil {
		panic(err)
	}
	return &servingCert
}

func main() {
	logger := slog.Default()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	ctx, ca := context.WithCancel(context.Background())
	defer ca()

	servingCert := loadCerts()

	relDB, err := sql.Open("sqlite3", "./otelfleet.db")
	if err != nil {
		logger.With("err", err.Error()).Error("failed to open relational store")
		os.Exit(1)
	}
	defer relDB.Close()
	logger.Info("embedded relational store started")

	kvDb, err := pebble.Open(
		"./otelfleet.kv",
		&pebble.Options{},
	)
	if err != nil {
		logger.Error("failed to start KV store")
		os.Exit(1)
	}

	defer func() {
		if err := kvDb.Close(); err != nil {
			logger.Error("failed to shutdown KV")
		}
	}()
	agentStore := otelpebble.NewPebbleBroker[*protobufs.AgentToServer](kvDb)
	tokenStore := otelpebble.NewPebbleBroker[*v1alpha1.BootstrapToken](kvDb)

	agentkv := agentStore.KeyValue("agents")
	tokenKv := tokenStore.KeyValue("tokens")

	r := gin.Default()

	srv := server.NewServer(logger.With("component", "opamp"))
	logger.Info("otelfleet starting...")

	bootstrapSrv := server.NewBootstrapServer(
		logger.With("component", "bootstrap"),
		servingCert.PrivateKey.(crypto.Signer),
		tokenKv,
		agentkv,
	)
	bootstrapSrv.ConfigureHttp(r)
	httpListenAddr := "127.0.0.1:8080"

	for _, route := range r.Routes() {
		logger.With("method", route.Method, "path", route.Path).Info("configured route")
	}
	go func() {
		logger.With("addr", httpListenAddr).Info("starting HTTP server...")
		if err := r.RunTLS(httpListenAddr, servingCertDataFile, servingKeyDataFile); err != nil {
			logger.With("err", err.Error()).Error("failed to start HTTP server")
			os.Exit(1)
		}
	}()
	if err := srv.Start(); err != nil {
		logger.Error("failed to start main otelfleet server")
	}
	logger.Info("otelfleet started")
	<-interrupt
	logger.Info("shutting down otelfleet")
	if err := srv.Stop(ctx); err != nil {
		logger.With("err", err).Error("failed to shutdown opamp server")
	}
}
