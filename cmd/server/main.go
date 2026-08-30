package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/otelfleet/otelfleet/pkg/config"
	_ "github.com/otelfleet/otelfleet/pkg/logutil"
	"github.com/otelfleet/otelfleet/pkg/server"
	"github.com/otelfleet/otelfleet/pkg/util/traceutil"
	"github.com/otelfleet/otelfleet/pkg/version"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

func BuildRootCommand() *cobra.Command {
	var configFilePath string
	cmd := &cobra.Command{
		Use: "otelfleet",
		Run: func(cmd *cobra.Command, args []string) {
			logger := slog.Default()
			cfg := &config.Config{}
			if configFilePath != "" {
				configData, err := os.ReadFile(configFilePath)
				if err != nil {
					logger.With("err", err).Error("failed to read config file")
					os.Exit(1)
				}
				if err := yaml.Unmarshal(configData, &cfg); err != nil {
					logger.With("err", err).Error("failed to decode config file")
					os.Exit(1)
				}
			}
			cfg.Sanitize()
			slog.SetDefault(cfg.SetupLogger())
			logger.With("version", version.FullVersion()).Info("starting otelfleet")

			if err := cfg.Validate(); err != nil {
				logger.With("err", err).Error("invalid config")
				os.Exit(1)
			}

			tp, err := traceutil.InitTracer(cmd.Context())
			if err != nil {
				logger.With("err", err).Error("failed to setup tracer provider")
				os.Exit(1)
			}
			otel.SetTracerProvider(tp)

			srv, err := server.New(*cfg, tp)
			if err != nil {
				logger.With("err", err).Error("failed to construct server")
				os.Exit(1)
			}

			if err := srv.Run(context.Background()); err != nil {
				logger.With("err", err).Error("failed to run server")
				os.Exit(1)
			}
		},
		Version: version.FullVersion(),
	}
	cmd.Flags().StringVarP(&configFilePath, "config", "f", "", "path to config file")
	return cmd
}

func main() {
	root := BuildRootCommand()

	if err := root.Execute(); err != nil {
		slog.With("err", err).Error("failed to run otelfleet")
	}
}
