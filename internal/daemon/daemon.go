// Package daemon manages the background sync loop.
package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/basmulder03/git-projects-sync/internal/config"
	"github.com/basmulder03/git-projects-sync/internal/logging"
	"github.com/basmulder03/git-projects-sync/internal/sync"
)

// Daemon runs the sync loop in the background.
type Daemon struct {
	cfg     *config.Config
	syncer  *sync.Syncer
	stopCh  chan struct{}
	pidFile string
}

// New creates a Daemon for the given config and config file path.
func New(cfg *config.Config, cfgPath string) *Daemon {
	home, _ := os.UserHomeDir()
	return &Daemon{
		cfg:     cfg,
		syncer:  sync.New(cfg, cfgPath),
		stopCh:  make(chan struct{}),
		pidFile: filepath.Join(home, ".git-sync", "daemon.pid"),
	}
}

// Start writes the PID file and begins the sync loop in a goroutine.
// Returns an error if the daemon is already running.
func (d *Daemon) Start() error {
	if d.IsRunning() {
		return fmt.Errorf("daemon already running (PID file: %s)", d.pidFile)
	}

	if err := os.MkdirAll(filepath.Dir(d.pidFile), 0o755); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}

	pid := os.Getpid()
	if err := os.WriteFile(d.pidFile, []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	interval, err := parseDuration(d.cfg.General.SyncInterval)
	if err != nil {
		interval = 15 * time.Minute
	}

	go d.loop(interval)
	logging.Info("daemon started", "pid", pid, "interval", interval.String())
	return nil
}

// Stop sends the stop signal and removes the PID file.
func (d *Daemon) Stop() error {
	close(d.stopCh)
	if err := os.Remove(d.pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove pid file: %w", err)
	}
	logging.Info("daemon stopped")
	return nil
}

// IsRunning returns true when a PID file exists and the recorded process is alive.
func (d *Daemon) IsRunning() bool {
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; Signal(0) probes liveness.
	// On Windows, FindProcess returns an error for dead processes.
	return isProcessAlive(proc)
}

// RunForeground runs the sync loop in the foreground until ctx is cancelled.
// Useful for debugging.
func RunForeground(cfg *config.Config) {
	syncer := sync.New(cfg, "")
	interval, err := parseDuration(cfg.General.SyncInterval)
	if err != nil {
		interval = 15 * time.Minute
	}

	ctx := context.Background()
	logging.Info("running sync in foreground", "interval", interval.String())
	for {
		if err := syncer.SyncAll(ctx); err != nil {
			logging.Error("sync error", "err", err)
		}
		time.Sleep(interval)
	}
}

func (d *Daemon) loop(interval time.Duration) {
	ctx := context.Background()
	for {
		if err := d.syncer.SyncAll(ctx); err != nil {
			logging.Error("sync error", "err", err)
		}
		select {
		case <-d.stopCh:
			return
		case <-time.After(interval):
		}
	}
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	return time.ParseDuration(s)
}
