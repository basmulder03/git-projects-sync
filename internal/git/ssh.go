// Package git provides Git operations and SSH config management.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/basmulder03/git-projects-sync/internal/config"
)

// EnsureSSHEntry creates or replaces the SSH config block for the given account.
// The block is delimited by marker comments so it can be updated idempotently.
func EnsureSSHEntry(account config.AccountConfig) error {
	p, err := sshConfigPath()
	if err != nil {
		return err
	}
	return ensureSSHEntryAt(account, p)
}

// RemoveSSHEntry removes the SSH config block for the given account ID.
func RemoveSSHEntry(accountID string) error {
	p, err := sshConfigPath()
	if err != nil {
		return err
	}
	return removeSSHEntryAt(accountID, p)
}

// ensureSSHEntryAt writes the block to a specific config file path (used in tests).
func ensureSSHEntryAt(account config.AccountConfig, configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return fmt.Errorf("ensure ssh dir: %w", err)
	}
	existing, err := readFileOrEmpty(configPath)
	if err != nil {
		return err
	}
	block := buildSSHBlock(account)
	updated := replaceMarkedBlock(existing, account.ID, block)
	return writeAtomic(configPath, updated)
}

// removeSSHEntryAt removes the block from a specific config file path (used in tests).
func removeSSHEntryAt(accountID, configPath string) error {
	existing, err := readFileOrEmpty(configPath)
	if err != nil {
		return err
	}
	updated := replaceMarkedBlock(existing, accountID, "")
	return writeAtomic(configPath, updated)
}

// sshConfigEntryExistsAt checks for a managed block at a specific config file path (used in tests).
func sshConfigEntryExistsAt(accountID, configPath string) (bool, error) {
	content, err := readFileOrEmpty(configPath)
	if err != nil {
		return false, err
	}
	return strings.Contains(content, "# git-sync-begin:"+accountID), nil
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

// SSHKeyExists returns true when the private key file at keyPath exists.
func SSHKeyExists(keyPath string) bool {
	_, err := os.Stat(config.ExpandPath(keyPath))
	return err == nil
}

// PublicKeyPath returns the expected public key path for a private key path.
func PublicKeyPath(privateKeyPath string) string {
	return config.ExpandPath(privateKeyPath) + ".pub"
}

// ReadPublicKey returns the contents of the public key file for the given private key path.
func ReadPublicKey(privateKeyPath string) (string, error) {
	pubPath := PublicKeyPath(privateKeyPath)
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("read public key %s: %w", pubPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// GenerateSSHKey generates an ed25519 SSH key pair at keyPath using ssh-keygen.
// Set overwrite=true to replace an existing key.
// SSHKeyTypeForProvider returns the SSH key type appropriate for a provider.
// Azure DevOps only accepts RSA keys; all other providers support ed25519.
func SSHKeyTypeForProvider(provider string) string {
	if provider == "azure_devops" {
		return "rsa"
	}
	return "ed25519"
}

func GenerateSSHKey(keyPath, comment string, overwrite bool, keyType string) error {
	expanded := config.ExpandPath(keyPath)

	if !overwrite {
		if _, err := os.Stat(expanded); err == nil {
			return fmt.Errorf("key already exists at %s (use --force to overwrite)", expanded)
		}
	}

	if err := os.MkdirAll(filepath.Dir(expanded), 0o700); err != nil {
		return fmt.Errorf("create key dir: %w", err)
	}

	// Remove existing key files so ssh-keygen does not prompt.
	_ = os.Remove(expanded)
	_ = os.Remove(expanded + ".pub")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if keyType == "" {
		keyType = "ed25519"
	}

	var stderr bytes.Buffer
	args := []string{"-t", keyType, "-C", comment, "-f", expanded, "-N", ""}
	if keyType == "rsa" {
		args = append([]string{"-b", "4096"}, args...)
	}
	cmd := exec.CommandContext(ctx, "ssh-keygen", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-keygen: %w\n%s", err, stderr.String())
	}
	return nil
}

// SSHConfigEntryExists returns true when the ~/.ssh/config file contains a managed block for accountID.
func SSHConfigEntryExists(accountID string) (bool, error) {
	p, err := sshConfigPath()
	if err != nil {
		return false, err
	}
	return sshConfigEntryExistsAt(accountID, p)
}

// SSHHostAlias returns the SSH host alias used in ~/.ssh/config for an account.
func SSHHostAlias(account config.AccountConfig) string {
	switch account.Provider {
	case "github":
		return "github.com-" + account.ID
	case "azure_devops":
		return "ssh.dev.azure.com-" + account.ID
	default:
		return account.Provider + "-" + account.ID
	}
}

// TestSSHConnection tests the SSH connection for the account.
// Returns the server's response output, whether authentication succeeded, and any exec error.
// Authentication success is determined by the response content, not the exit code
// (GitHub returns exit 1 on successful auth; AzDo returns exit 128).
func TestSSHConnection(account config.AccountConfig) (string, bool, error) {
	alias := SSHHostAlias(account)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "ssh", "-T",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"git@"+alias,
	)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	_ = cmd.Run() // exit code is non-zero even on success for GitHub/AzDo

	out := strings.TrimSpace(buf.String())
	success := isSSHAuthSuccess(account.Provider, out)
	return out, success, nil
}

// ProviderKeyURL returns the URL where the user should add their public key.
func ProviderKeyURL(account config.AccountConfig) string {
	switch account.Provider {
	case "github":
		return "https://github.com/settings/ssh/new"
	case "azure_devops":
		org := account.Organization
		if org == "" {
			org = account.Username
		}
		return fmt.Sprintf("https://dev.azure.com/%s/_usersSettings/keys", org)
	default:
		return ""
	}
}

func isSSHAuthSuccess(provider, output string) bool {
	lower := strings.ToLower(output)
	switch provider {
	case "github":
		// "Hi username! You've successfully authenticated"
		return strings.Contains(lower, "successfully authenticated") || strings.Contains(lower, "hi ")
	case "azure_devops":
		// "remote: Shell access is not supported." means auth worked
		return strings.Contains(lower, "shell access is not supported") ||
			strings.Contains(lower, "successfully authenticated")
	default:
		return strings.Contains(lower, "successfully authenticated")
	}
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
