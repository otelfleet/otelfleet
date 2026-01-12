package supervisor

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/natefinch/atomic"
	"github.com/open-telemetry/opamp-go/protobufs"
)

type ProcManager struct {
	logger     *slog.Logger
	BinaryPath string
	ConfigDir  string

	runMu     *sync.Mutex
	cmd       *exec.Cmd
	cmdExited chan struct{}
	curHash   []byte

	reportHealthFn func(
		healthy bool,
		status string,
		lastErrorMessage string,
	)
}

func NewProcManager(
	logger *slog.Logger,
	binaryPath,
	configPath string,
	reportFn func(bool, string, string),
) *ProcManager {
	return &ProcManager{
		runMu:          &sync.Mutex{},
		logger:         logger,
		BinaryPath:     binaryPath,
		ConfigDir:      configPath,
		reportHealthFn: reportFn,
	}
}

func (p *ProcManager) Update(
	ctx context.Context,
	incoming *protobufs.AgentRemoteConfig,
) error {
	p.runMu.Lock()
	defer p.runMu.Unlock()

	if bytes.Equal([]byte(p.curHash), incoming.GetConfigHash()) {
		p.logger.Info("got identical config, skipping update")
		return nil
	}

	return p.runLocked(ctx, incoming)
}

func (p *ProcManager) runLocked(ctx context.Context, incoming *protobufs.AgentRemoteConfig) error {
	hashPath := path.Join(p.ConfigDir, "config.hash")
	if err := os.WriteFile(hashPath, incoming.GetConfigHash(), 0644); err != nil {
		return err
	}
	// TODO : this doens't handle cleanup of dangling names
	configMap := incoming.GetConfig().GetConfigMap()
	for name, contents := range configMap {
		if err := p.writeConfigLocked(name, contents); err != nil {
			return err
		}
	}
	args := []string{}
	for name := range configMap {
		args = append(
			args,
			"--config",
			path.Join(p.ConfigDir, name),
		)
	}
	if len(args) == 0 {
		panic("0 configs not handled")
	}
	p.releaseLocked()
	p.logger.With("binary", p.BinaryPath, "args", strings.Join(args, " ")).Info("executing command...")
	cmd := exec.Command(p.BinaryPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe for envoy: %w", err)
	}
	go p.handleLogs(ctx, stderr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe for envoy : %w", err)
	}
	go p.handleLogs(ctx, stdout)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		// Pdeathsig: shutdownSignal,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting collector")
	}
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		err := cmd.Wait()
		p.logger.With("exit-status", err).Info("command exited")
		if err != nil {
			p.logger.Info("reporting failure to opamp server")
			p.reportHealthFn(false, fmt.Sprintf("collector exited : %s", err), "TODO : last error message")
		}
	}()

	// is there a ready check for otelcol collector we can
	// leverage here, or just health?
	p.cmd = cmd
	p.cmdExited = exited
	return nil
}

func (p *ProcManager) handleLogs(ctx context.Context, rc io.ReadCloser) {
	defer rc.Close()

	l := p.logger.With("service", "otelcol")
	bo := backoff.NewExponentialBackOff()

	s := bufio.NewReader(rc)
	for {
		ln, err := s.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				break
			}
			l.With("err", err).Error("failed to read log")
			time.Sleep(bo.NextBackOff())
			continue
		}
		ln = strings.TrimRight(ln, "\r\n")
		bo.Reset()

		if ln == "" {
			continue
		}

		// lvl, msg, attrs := p.parseOtelcolLog(ln)
		// l.LogAttrs(ctx, lvl, msg, attrs...)
	}
}

func (p *ProcManager) Shutdown() error {
	// TODO:
	if p.cmd != nil && p.cmd.Process != nil {
		gracefulShutdown := time.Minute
		_ = p.cmd.Process.Signal(shutdownSignal)
		select {
		case <-p.cmdExited:
			return nil
		case <-time.After(gracefulShutdown):

		}
		if err := p.cmd.Process.Kill(); err != nil {
			p.logger.With("err", err).Error("failed to kill the process")
		} else {
			<-p.cmdExited
		}
		p.cmd = nil

	}
	return nil
}

func (p *ProcManager) releaseLocked() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.logger.Info("releasing collector process")
		if err := p.cmd.Process.Release(); err != nil {
			p.logger.With("err", err).Error("releasing process")
		}
	}
}

func (p *ProcManager) writeConfigLocked(name string, config *protobufs.AgentConfigFile) error {
	fileName := path.Join(p.ConfigDir, name)
	p.logger.With("file", fileName).Info("writing config file")
	if err := atomic.WriteFile(fileName, bytes.NewReader(config.GetBody())); err != nil {
		return err
	}
	return nil
}

func (p *ProcManager) getConfigMap() (*protobufs.AgentConfigMap, error) {
	entries, err := os.ReadDir(p.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("reading config directory: %w", err)
	}

	configMap := make(map[string]*protobufs.AgentConfigFile)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip the hash file
		if name == "config.hash" {
			continue
		}

		filePath := path.Join(p.ConfigDir, name)
		body, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", name, err)
		}

		contentType := guessContentType(name)
		configMap[name] = &protobufs.AgentConfigFile{
			Body:        body,
			ContentType: contentType,
		}
	}

	return &protobufs.AgentConfigMap{ConfigMap: configMap}, nil
}

func guessContentType(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".json":
		return "application/json"
	case ".toml":
		return "application/toml"
	default:
		return "text/plain"
	}
}
