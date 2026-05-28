//go:build linux

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const systemdService = "git-sync.service"

func registerAutostart(exePath string, system bool) error {
	unitPath, err := systemdUnitPath(system)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("create systemd dir: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(systemdUnitContent(exePath, system)), 0o644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	return systemctlEnable(system)
}

func unregisterAutostart(system bool) error {
	_ = systemctlDisable(system)
	unitPath, err := systemdUnitPath(system)
	if err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isAutostartRegistered(system bool) (bool, error) {
	unitPath, err := systemdUnitPath(system)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(unitPath)
	return err == nil, nil
}

func systemdUnitPath(system bool) (string, error) {
	if system {
		return "/etc/systemd/system/" + systemdService, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdService), nil
}

func systemdUnitContent(exePath string, system bool) string {
	wantedBy := "default.target"
	if system {
		wantedBy = "multi-user.target"
	}
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=git-sync background sync daemon\nAfter=network-online.target\n\n")
	b.WriteString("[Service]\nType=simple\n")
	b.WriteString("ExecStart=" + exePath + " daemon start\n")
	b.WriteString("Restart=on-failure\nRestartSec=30\n\n")
	b.WriteString("[Install]\nWantedBy=" + wantedBy + "\n")
	return b.String()
}

func systemctlEnable(system bool) error {
	args := []string{"enable", "--now", systemdService}
	if !system {
		args = append([]string{"--user"}, args...)
	}
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl enable: %w\n%s", err, out)
	}
	return nil
}

func systemctlDisable(system bool) error {
	args := []string{"disable", "--now", systemdService}
	if !system {
		args = append([]string{"--user"}, args...)
	}
	_ = exec.Command("systemctl", args...).Run()
	return nil
}
