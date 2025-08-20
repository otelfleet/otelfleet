package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"database/sql"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/otelfleet/otelfleet/pkg/logger"
	"github.com/otelfleet/otelfleet/pkg/server"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func main() {
	logger := slog.Default()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	db, err := sql.Open("sqlite3", "./otelfleet.db")
	if err != nil {
		logger.With("err", err.Error()).Error("failed to open database")
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("embedded database started")

	r := gin.Default()

	srv := server.NewServer(logger.With("component", "opamp"))
	logger.Info("otelfleet starting...")
	srv.Start()
	httpListenAddr := "127.0.0.1:8080"
	go func() {
		logger.With("addr", httpListenAddr).Info("starting HTTP server...")
		if err := r.Run(httpListenAddr); err != nil {
			logger.With("err", err.Error()).Error("failed to start HTTP server")
			os.Exit(1)
		}
	}()
	logger.Info("otelfleet started")
	<-interrupt
	logger.Info("shutting down otelfleet")
	srv.Stop(context.Background())
}
