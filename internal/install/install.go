// Package install handles binary deployment, autostart registration, and privilege escalation.
package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Options controls install behaviour.
type Options struct {
	System      bool // system-wide install — requires elevation
	NoAutostart bool // skip autostart registration
}

// Install copies the running binary to the install location and (optionally) registers autostart.
func Install(opts Options) error {
	if opts.System && !isElevated() {
		fmt.Println("System install requires elevated privileges — requesting elevation...")
		return relaunchElevated()
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = real
	}

	targetDir, err := InstallDir(opts.System)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create install dir %s: %w", targetDir, err)
	}

	targetExe := filepath.Join(targetDir, ExeName())

	if same, _ := sameFile(exePath, targetExe); !same {
		if err := copyFile(exePath, targetExe); err != nil {
			return fmt.Errorf("copy binary to %s: %w", targetExe, err)
		}
		fmt.Printf("Installed → %s\n", targetExe)
	} else {
		fmt.Printf("Binary already at %s\n", targetExe)
	}

	if err := ensureInPath(targetDir, opts.System); err != nil {
		fmt.Printf("  Note: add %s to PATH manually\n", targetDir)
	}

	if !opts.NoAutostart {
		if err := registerAutostart(targetExe, opts.System); err != nil {
			return fmt.Errorf("register autostart: %w", err)
		}
		fmt.Println("Autostart registered — daemon launches at login")
	}

	return nil
}

// Uninstall removes the autostart registration (does not delete the binary).
func Uninstall(opts Options) error {
	if opts.System && !isElevated() {
		fmt.Println("System uninstall requires elevated privileges — requesting elevation...")
		return relaunchElevated()
	}
	if err := unregisterAutostart(opts.System); err != nil {
		return fmt.Errorf("unregister autostart: %w", err)
	}
	fmt.Println("Autostart removed")
	return nil
}

// IsAutostartRegistered reports whether autostart is currently configured.
func IsAutostartRegistered(system bool) (bool, error) {
	return isAutostartRegistered(system)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func sameFile(a, b string) (bool, error) {
	ia, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(ia, ib), nil
}
