package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	stdpath "path"
	"slices"
	"strings"
	"time"

	"github.com/otelfleet/otelfleet/pkg/logutil"
)

type Config struct {
	LogConfig         *LogConfig     `yaml:"log,omitempty"`
	Services          []string       `yaml:"services"`
	HttpListenAddr    string         `yaml:"http_listen_addr"`
	HttpListenNetwork string         `yaml:"http_listen_network"`
	GRPCListenAddr    string         `yaml:"grpc_listen_addr"`
	Certificates      *CertConfig    `yaml:"certs,omitempty"`
	StorageConfig     *StorageConfig `yaml:"storage,omitempty"`
	UI                *UIConfig      `yaml:"ui,omitempty"`
	OTLP              *OTLPConfig    `yaml:"otlp,omitempty"`
	LSP               *LSPConfig     `yaml:"lsp,omitempty"`
}

// Sanitize sets sane required defaults if none are present
func (c *Config) Sanitize() {
	if c.LogConfig == nil {
		c.LogConfig = &LogConfig{
			Level:  "info",
			Format: "json",
		}
	}
	c.LogConfig.Sanitize()
	if c.HttpListenAddr == "" {
		c.HttpListenAddr = "0.0.0.0:16587"
	}
	if c.HttpListenNetwork == "" {
		c.HttpListenNetwork = "tcp4"
	}
	if c.GRPCListenAddr == "" {
		c.GRPCListenAddr = "0.0.0.0:16586"
	}
	if len(c.Services) == 0 {
		c.Services = []string{"all"}
	}
	if c.StorageConfig == nil {
		c.StorageConfig = &StorageConfig{
			File: &StorageConfigFilesystem{
				Path: "./otelfleet.kv",
			},
		}
	}
	if c.UI == nil {
		c.UI = &UIConfig{}
	}
	if c.OTLP == nil {
		c.OTLP = &OTLPConfig{}
	}
	if c.LSP == nil {
		c.LSP = &LSPConfig{}
	}

	c.UI.Sanitize()
	c.LSP.Sanitize()
	c.OTLP.Sanitize()
}

func (c *Config) SetupLogger() *slog.Logger {
	return logutil.NewLogger(c.LogConfig.Level, c.LogConfig.Format)
}

func (c *Config) Validate() error {
	if err := c.LogConfig.Validate(); err != nil {
		return err
	}
	if c.HttpListenAddr == "" {
		return errors.New("http listen address must be set")
	}

	if c.GRPCListenAddr == "" {
		return errors.New("grpc listen addr must be set")
	}

	if c.StorageConfig == nil {
		return errors.New("storage must be set")
	}
	if err := c.StorageConfig.Validate(); err != nil {
		return err
	}
	if c.UI != nil {
		if err := c.UI.Validate(); err != nil {
			return err
		}
	}
	if c.Certificates != nil {
		if err := c.Certificates.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type LogConfig struct {
	Level  string
	Format string
	// TODO : as otelfleet gets closer to a stable version, have per service/ service component logger configurations.
}

func (c *LogConfig) Sanitize() {
	c.Level = strings.ToLower(c.Level)
	c.Format = strings.ToLower(c.Format)
}

func (c *LogConfig) Validate() error {
	if !slices.Contains([]string{
		"debug",
		"info",
		"warn",
		"error",
	}, c.Level) {
		return fmt.Errorf("invalid log level : %s", c.Level)
	}
	if !slices.Contains([]string{
		"color",
		"json",
		"none",
	}, c.Format) {
		return fmt.Errorf("invalid log format : %s", c.Format)
	}
	return nil
}

type CertConfig struct {
	Server     ServerCertConfig    `yaml:"server"`
	Collectors CollectorCertConfig `yaml:"collectors"`
}

func (c *CertConfig) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return err
	}
	if err := c.Collectors.Validate(); err != nil {
		return err
	}
	return nil
}

type ServerCertConfig struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func (c *ServerCertConfig) Validate() error {
	if c.CertFile == "" || c.KeyFile == "" {
		return errors.New("server certificate and key are required")
	}
	return nil
}

type CollectorCertConfig struct {
	CaCertFile  string        `yaml:"ca_cert_file"`
	CaKeyFile   string        `yaml:"ca_key_file"`
	ValidFor    time.Duration `yaml:"valid_for"`
	RenewBefore time.Duration `yaml:"renew_before"`
}

func (c *CollectorCertConfig) Validate() error {
	if c.CaCertFile == "" || c.CaKeyFile == "" {
		return errors.New("collector CA certificate and key are required")
	}
	if c.ValidFor <= 0 {
		return errors.New("collector certificate valid_for must be positive")
	}
	if c.RenewBefore <= 0 || c.RenewBefore >= c.ValidFor {
		return errors.New("renew_before must be positive and less than valid_for")
	}
	return nil
}

type UIConfig struct {
	ApiURL     string `yaml:"api_url,omitempty"`
	PathPrefix string `yaml:"path_prefix,omitempty"`
}

func (u *UIConfig) Sanitize() {
	if u.PathPrefix == "" {
		u.PathPrefix = "/ui"
	}
}

func (u *UIConfig) Validate() error {
	if u.ApiURL != "" {
		parsed, err := url.Parse(u.ApiURL)
		if err != nil {
			return errors.New("ui upstream is not a valid URL")
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return errors.New("ui upstream must be an absolute URL (scheme and host)")
		}
	}
	return nil
}

type StorageConfig struct {
	File   *StorageConfigFilesystem `yaml:"file,omitempty"`
	Client *StorageConfigClient     `yaml:"client,omitempty"`
}

func (s *StorageConfig) Validate() error {
	if s.File == nil && s.Client == nil {
		return errors.New("storage must be set")
	}
	if s.File != nil && s.Client != nil {
		return errors.New("file must be set")
	}
	if s.File != nil {
		if err := s.File.Validate(); err != nil {
			return err
		}
	}
	if s.Client != nil {
		if err := s.Client.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type StorageConfigFilesystem struct {
	Path string `yaml:"path"`
}

func (s *StorageConfigFilesystem) Validate() error {
	if s.Path == "" {
		return errors.New("storage file path must be set")
	}
	return nil
}

type StorageConfigClient struct {
	HttpAddr string `yaml:"http_addr"`
	// TODO : other client stuff
}

func (s *StorageConfigClient) Validate() error {
	if s.HttpAddr == "" {
		return errors.New("http address must be set")
	}
	return nil
}

type OTLPConfig struct {
	ListenAddr     string `yaml:"listen_address"`
	AdvertiseAddr  string `yaml:"advertise_addr,omitempty"`
	BasePath       string `yaml:"base_path"`
	MetricsAPIPath string `yaml:"metrics_api_path"`
	LogsAPIPath    string `yaml:"logs_api_path"`
	TraceAPIPath   string `yaml:"trace_api_path"`
}

func (c *OTLPConfig) Sanitize() {
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:10200"
	}
	if c.AdvertiseAddr == "" {
		c.AdvertiseAddr = "otelfeet.io"
	}
	if c.BasePath == "" {
		c.BasePath = "/otlp"
	}
	if c.MetricsAPIPath == "" {
		c.MetricsAPIPath = "/v1/metrics"
	}
	if c.LogsAPIPath == "" {
		c.LogsAPIPath = "/v1/logs"
	}
	if c.TraceAPIPath == "" {
		c.TraceAPIPath = "/v1/trace"
	}
}

func (c *OTLPConfig) Validate() error {
	if err := validateAPIPath(c.BasePath); err != nil {
		return fmt.Errorf("base api path : %w", err)
	}
	if err := validateAPIPath(c.MetricsAPIPath); err != nil {
		return fmt.Errorf("metrics api path: %w", err)
	}
	if err := validateAPIPath(c.LogsAPIPath); err != nil {
		return fmt.Errorf("logs api path: %w", err)
	}
	if err := validateAPIPath(c.TraceAPIPath); err != nil {
		return fmt.Errorf("trace api path: %w", err)
	}
	if c.MetricsAPIPath == c.LogsAPIPath ||
		c.MetricsAPIPath == c.TraceAPIPath ||
		c.LogsAPIPath == c.TraceAPIPath {
		return errors.New("otlp api paths must be distinct")
	}
	return nil
}

func validateAPIPath(path string) error {
	if !strings.HasPrefix(path, "/") {
		return errors.New("must start with '/'")
	}
	if strings.ContainsAny(path, "?#") {
		return errors.New("must not contain a query or fragment")
	}
	if strings.ContainsFunc(path, func(r rune) bool {
		return r <= ' ' || r == 0x7f
	}) {
		return errors.New("must not contain whitespace or control characters")
	}
	unescaped, err := url.PathUnescape(path)
	if err != nil {
		return errors.New("contains an invalid percent-encoded sequence")
	}
	if unescaped != path {
		return errors.New("must not be percent-encoded")
	}
	if cleaned := stdpath.Clean(path); cleaned != path {
		return fmt.Errorf("must be a clean path (did you mean %q?)", cleaned)
	}
	return nil
}

type LSPConfig struct {
	// DistCache path to the dist cache required by the otelcol-lsp
	DistCache string `yaml:"dist_cache"`
	// Default distribution for LSP if none is specified. i.e. otelcol
	DefaultDistributionName string `yaml:"default_distribution"`
	// Default distribution version for LSP if none is specified i.e. 0.156.0
	DefaultDistributionVersion string `yaml:"default_version"`
}

func (c *LSPConfig) Validate() error {
	return nil
}

func (c *LSPConfig) Sanitize() {
	if c.DistCache == "" {
		c.DistCache = "/var/lib/cache/otelconf"
	}
	if c.DefaultDistributionName == "" {
		c.DefaultDistributionName = "otelcol-contrib"
	}
	if c.DefaultDistributionVersion == "" {
		c.DefaultDistributionVersion = "0.157.0"
	}
}
