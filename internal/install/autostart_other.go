//go:build !windows && !linux && !darwin

package install

import "fmt"

func registerAutostart(_ string, _ bool) error {
	return fmt.Errorf("autostart registration not supported on this platform")
}

func unregisterAutostart(_ bool) error { return nil }

func isAutostartRegistered(_ bool) (bool, error) { return false, nil }
