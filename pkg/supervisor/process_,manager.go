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
}

func NewProcManager(
	logger *slog.Logger,
	binaryPath,
	configPath string,
) *ProcManager {
	return &ProcManager{
		runMu:      &sync.Mutex{},
		logger:     logger,
		BinaryPath: binaryPath,
		ConfigDir:  configPath,
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
		// TODO : parse and reformat log

		lvl := p.slogLevelFromOtelcol(ln)

		if ln == "" {
			continue
		}
		l.Log(ctx, lvl, ln)
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

func (p *ProcManager) slogLevelFromOtelcol(_ string) slog.Level {
	// TODO: implement me
	return slog.LevelInfo
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
	if err := atomic.WriteFile(fileName, bytes.NewReader(config.GetBody())); err != nil {
		return err
	}
	return nil
}
