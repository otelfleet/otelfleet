package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otelfleet/otelfleet/pkg/config"
	_ "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/server"
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
	srv, err := server.New(config.Config{
		StoragePath: "./otelfleet.kv",
	})
	if err != nil {
		logger.With("err", err).Error("failed to construct server")
		os.Exit(1)
	}

	if err := srv.Run(context.Background()); err != nil {
		logger.With("err", err).Error("failed to run server")
		os.Exit(1)
	}
	// logger := slog.Default()
	// interrupt := make(chan os.Signal, 1)
	// signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// ctx, ca := context.WithCancel(context.Background())
	// defer ca()

	// servingCert := loadCerts()

	// relDB, err := sql.Open("sqlite3", "./otelfleet.db")
	// if err != nil {
	// 	logger.With("err", err.Error()).Error("failed to open relational store")
	// 	os.Exit(1)
	// }
	// defer relDB.Close()
	// logger.Info("embedded relational store started")

	// kvDb, err := pebble.Open(
	// 	"./otelfleet.kv",
	// 	&pebble.Options{},
	// )
	// if err != nil {
	// 	logger.Error("failed to start KV store")
	// 	os.Exit(1)
	// }

	// defer func() {
	// 	if err := kvDb.Close(); err != nil {
	// 		logger.Error("failed to shutdown KV")
	// 	}
	// }()
	// agentStore := otelpebble.NewPebbleBroker[*protobufs.AgentToServer](kvDb)
	// tokenStore := otelpebble.NewPebbleBroker[*v1alpha1.BootstrapToken](kvDb)

	// agentkv := agentStore.KeyValue("agents")
	// tokenKv := tokenStore.KeyValue("tokens")

	// r := gin.Default()

	// uiHandler, err := ui.ServeUI()
	// if err != nil {
	// 	logger.With("err", err).Error("failed to embed UI")
	// }

	// an := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	logger.With("path", r.URL.Path).Info("ui handler")
	// 	uiHandler.ServeHTTP(w, r)
	// })

	// r.Any("/ui/*filepath", gin.WrapH(http.StripPrefix("/ui", an)))

	// r.NoRoute(func(c *gin.Context) {
	// 	ref := c.Request.Referer()

	// 	logger.With("referrer", ref).With("path", c.Request.URL.Path).Debug("deciding whether to re-proxy to UI...")
	// 	if ref != "" && strings.Contains(ref, "/ui") {
	// 		r := c.Request
	// 		r.URL.Path = "/ui" + r.URL.Path
	// 		http.StripPrefix("/ui", uiHandler).ServeHTTP(c.Writer, r)
	// 		c.Abort()
	// 		return
	// 	}

	// 	c.Status(http.StatusNotFound)
	// })

	// srv := server.NewServer(logger.With("component", "opamp"))
	// logger.Info("otelfleet starting...")

	// bootstrapSrv := server.NewBootstrapServer(
	// 	logger.With("component", "bootstrap"),
	// 	servingCert.PrivateKey.(crypto.Signer),
	// 	tokenKv,
	// 	agentkv,
	// )
	// bootstrapSrv.ConfigureHttp(r)
	// httpListenAddr := "127.0.0.1:8080"

	// for _, route := range r.Routes() {
	// 	logger.With("method", route.Method, "path", route.Path).Info("configured route")
	// }
	// go func() {
	// 	logger.With("addr", httpListenAddr).Info("starting HTTP server...")
	// 	if err := r.RunTLS(httpListenAddr, servingCertDataFile, servingKeyDataFile); err != nil {
	// 		logger.With("err", err.Error()).Error("failed to start HTTP server")
	// 		os.Exit(1)
	// 	}
	// }()
	// if err := srv.Start(); err != nil {
	// 	logger.Error("failed to start main otelfleet server")
	// }
	// logger.Info("otelfleet started")
	// <-interrupt
	// logger.Info("shutting down otelfleet")
	// if err := srv.Stop(ctx); err != nil {
	// 	logger.With("err", err).Error("failed to shutdown opamp server")
	// }
}
