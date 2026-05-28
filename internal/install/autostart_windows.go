//go:build windows

package install

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath    = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartName = "git-sync"
)

func registerAutostart(exePath string, system bool) error {
	k, _, err := registry.CreateKey(autostartRoot(system), runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open autostart registry key: %w", err)
	}
	defer k.Close()
	return k.SetStringValue(autostartName, `"`+exePath+`" daemon start`)
}

func unregisterAutostart(system bool) error {
	k, err := registry.OpenKey(autostartRoot(system), runKeyPath, registry.SET_VALUE)
	if err != nil {
		return nil // key does not exist — nothing to remove
	}
	defer k.Close()
	if err := k.DeleteValue(autostartName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

func isAutostartRegistered(system bool) (bool, error) {
	k, err := registry.OpenKey(autostartRoot(system), runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, nil
	}
	defer k.Close()
	_, _, err = k.GetStringValue(autostartName)
	return err == nil, nil
}

func autostartRoot(system bool) registry.Key {
	if system {
		return registry.LOCAL_MACHINE
	}
	return registry.CURRENT_USER
}
