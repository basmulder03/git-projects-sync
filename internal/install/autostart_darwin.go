//go:build darwin

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	plistLabel = "com.github.basmulder03.git-sync"
	plistFile  = plistLabel + ".plist"
)

func registerAutostart(exePath string, system bool) error {
	dir, err := plistDir(system)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}
	plistPath := filepath.Join(dir, plistFile)
	if err := os.WriteFile(plistPath, []byte(launchPlist(exePath)), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	return launchctlLoad(plistPath)
}

func unregisterAutostart(system bool) error {
	dir, err := plistDir(system)
	if err != nil {
		return err
	}
	plistPath := filepath.Join(dir, plistFile)
	launchctlUnload(plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func isAutostartRegistered(system bool) (bool, error) {
	dir, err := plistDir(system)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(dir, plistFile))
	return err == nil, nil
}

func plistDir(system bool) (string, error) {
	if system {
		return "/Library/LaunchDaemons", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

func launchPlist(exePath string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>` + plistLabel + `</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + exePath + `</string>
        <string>daemon</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
</dict>
</plist>
`
}

func launchctlLoad(plistPath string) error {
	out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %w\n%s", err, out)
	}
	return nil
}

func launchctlUnload(plistPath string) {
	_ = exec.Command("launchctl", "unload", plistPath).Run()
}
