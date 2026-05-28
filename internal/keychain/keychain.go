// Package keychain wraps the OS keychain for storing PAT tokens.
package keychain

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const service = "git-sync"

// SetPAT stores a PAT for the given account ID in the OS keychain.
func SetPAT(accountID, token string) error {
	if err := keyring.Set(service, patKey(accountID), token); err != nil {
		return fmt.Errorf("keychain set PAT %s: %w", accountID, err)
	}
	return nil
}

// GetPAT retrieves the PAT for the given account ID from the OS keychain.
func GetPAT(accountID string) (string, error) {
	token, err := keyring.Get(service, patKey(accountID))
	if err != nil {
		return "", fmt.Errorf("keychain get PAT %s: %w", accountID, err)
	}
	return token, nil
}

// DeletePAT removes the PAT for the given account ID from the OS keychain.
func DeletePAT(accountID string) error {
	if err := keyring.Delete(service, patKey(accountID)); err != nil {
		return fmt.Errorf("keychain delete PAT %s: %w", accountID, err)
	}
	return nil
}

func patKey(accountID string) string {
	return "pat:" + accountID
}
