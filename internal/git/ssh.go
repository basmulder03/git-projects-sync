// Package git provides Git operations and SSH config management.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/basmulder03/git-projects-sync/internal/config"
)

// EnsureSSHEntry creates or replaces the SSH config block for the given account.
// The block is delimited by marker comments so it can be updated idempotently.
func EnsureSSHEntry(account config.AccountConfig) error {
	sshConfigPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(sshConfigPath), 0o700); err != nil {
		return fmt.Errorf("ensure ssh dir: %w", err)
	}

	existing, err := readFileOrEmpty(sshConfigPath)
	if err != nil {
		return err
	}

	block := buildSSHBlock(account)
	updated := replaceMarkedBlock(existing, account.ID, block)

	return writeAtomic(sshConfigPath, updated)
}

// RemoveSSHEntry removes the SSH config block for the given account ID.
func RemoveSSHEntry(accountID string) error {
	sshConfigPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	existing, err := readFileOrEmpty(sshConfigPath)
	if err != nil {
		return err
	}

	updated := replaceMarkedBlock(existing, accountID, "")
	return writeAtomic(sshConfigPath, updated)
}

func sshConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func buildSSHBlock(account config.AccountConfig) string {
	keyPath := config.ExpandPath(account.SSHKeyPath)

	var host, hostName string
	switch account.Provider {
	case "github":
		host = "github.com-" + account.ID
		hostName = "github.com"
	case "azure_devops":
		host = "ssh.dev.azure.com-" + account.ID
		hostName = "ssh.dev.azure.com"
	default:
		host = account.Provider + "-" + account.ID
		hostName = account.Provider
	}

	return fmt.Sprintf(
		"Host %s\n    HostName %s\n    User git\n    IdentityFile %s\n    IdentitiesOnly yes\n",
		host, hostName, keyPath,
	)
}

// replaceMarkedBlock replaces content between begin/end markers or appends the block.
// An empty block string removes the marked section.
func replaceMarkedBlock(content, accountID, block string) string {
	beginMarker := "# git-sync-begin:" + accountID
	endMarker := "# git-sync-end:" + accountID

	beginIdx := strings.Index(content, beginMarker)
	endIdx := strings.Index(content, endMarker)

	if beginIdx != -1 && endIdx != -1 && endIdx > beginIdx {
		// Replace existing block including markers and trailing newline.
		endLineEnd := endIdx + len(endMarker)
		if endLineEnd < len(content) && content[endLineEnd] == '\n' {
			endLineEnd++
		}
		before := content[:beginIdx]
		after := content[endLineEnd:]

		if block == "" {
			// Strip a trailing newline from before if we're removing the block.
			before = strings.TrimRight(before, "\n")
			if len(before) > 0 {
				before += "\n"
			}
			return before + after
		}
		return before + beginMarker + "\n" + block + endMarker + "\n" + after
	}

	if block == "" {
		return content
	}

	// Append new block.
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + beginMarker + "\n" + block + endMarker + "\n"
}

func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read ssh config: %w", err)
	}
	return string(data), nil
}

func writeAtomic(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write ssh config tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename ssh config: %w", err)
	}
	return nil
}
