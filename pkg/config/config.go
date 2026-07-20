package config

import (
	"errors"
	"net/url"
)

type Config struct {
	Services       []string       `yaml:"services"`
	HttpListenAddr string         `yaml:"http_listen_addr"`
	StorageConfig  *StorageConfig `yaml:"storage"`
	UI             *UIConfig      `yaml:"ui,omitempty"`
}

// Sanitize sets sane required defaults if none are present
func (c *Config) Sanitize() {
	if c.HttpListenAddr == "" {
		c.HttpListenAddr = "127.0.0.1:16587"
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
	c.UI.Sanitize()
}

func (c *Config) Validate() error {
	if c.HttpListenAddr == "" {
		return errors.New("http listen address must be set")
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
