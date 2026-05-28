//go:build !windows

package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ExeName returns the binary filename for this platform.
func ExeName() string { return "git-sync" }

// InstallDir returns the target directory for the binary.
func InstallDir(system bool) (string, error) {
	if system {
		return "/usr/local/bin", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home: %w", err)
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func ensureInPath(dir string, _ bool) error {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return nil // already in current PATH
		}
	}

	export := fmt.Sprintf("\nexport PATH=\"%s:$PATH\"\n", dir)
	for _, f := range profileFiles() {
		appendIfAbsent(f, dir, export)
	}
	fmt.Printf("Added %s to PATH in profile — restart your terminal\n", dir)
	return nil
}

func profileFiles() []string {
	home, _ := os.UserHomeDir()
	files := []string{filepath.Join(home, ".profile")}
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		files = append(files, filepath.Join(home, ".zshrc"))
	case "bash":
		files = append(files, filepath.Join(home, ".bashrc"))
	}
	return files
}

func appendIfAbsent(filePath, marker, content string) {
	data, _ := os.ReadFile(filePath)
	if strings.Contains(string(data), marker) {
		return
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(content)
}

func isElevated() bool {
	return os.Getuid() == 0
}

func relaunchElevated() error {
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo not found — re-run as root: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	argv := append([]string{sudo, exe}, os.Args[1:]...)
	return syscall.Exec(sudo, argv, os.Environ())
}
