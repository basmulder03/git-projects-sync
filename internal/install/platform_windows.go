//go:build windows

package install

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ExeName returns the binary filename for this platform.
func ExeName() string { return "git-sync.exe" }

// InstallDir returns the target directory for the binary.
func InstallDir(system bool) (string, error) {
	if system {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = `C:\Program Files`
		}
		return pf + `\git-sync`, nil
	}
	appdata := os.Getenv("LOCALAPPDATA")
	if appdata == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home: %w", err)
		}
		appdata = home + `\AppData\Local`
	}
	return appdata + `\Programs\git-sync`, nil
}

func ensureInPath(dir string, system bool) error {
	var root registry.Key
	var keyPath string
	if system {
		root = registry.LOCAL_MACHINE
		keyPath = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
	} else {
		root = registry.CURRENT_USER
		keyPath = `Environment`
	}

	k, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open PATH key: %w", err)
	}
	defer k.Close()

	current, valtype, err := k.GetStringValue("Path")
	if err != nil {
		current = ""
		valtype = registry.EXPAND_SZ
	}

	for _, p := range strings.Split(current, ";") {
		if strings.EqualFold(strings.TrimSpace(p), dir) {
			return nil // already present
		}
	}

	newPath := current
	if newPath != "" && !strings.HasSuffix(newPath, ";") {
		newPath += ";"
	}
	newPath += dir

	if valtype == registry.EXPAND_SZ {
		err = k.SetExpandStringValue("Path", newPath)
	} else {
		err = k.SetStringValue("Path", newPath)
	}
	if err != nil {
		return fmt.Errorf("write PATH: %w", err)
	}

	broadcastSettingChange()
	fmt.Printf("Added %s to PATH (restart your terminal)\n", dir)
	return nil
}

func broadcastSettingChange() {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMessageTimeout := user32.NewProc("SendMessageTimeoutW")
	env := syscall.StringToUTF16Ptr("Environment")
	sendMessageTimeout.Call(
		0xffff,                        // HWND_BROADCAST
		0x001a,                        // WM_SETTINGCHANGE
		0,                             // wParam
		uintptr(unsafe.Pointer(env)), // lParam = "Environment"
		0x0002,                        // SMTO_ABORTIFHUNG
		5000,
		0,
	)
}

func isElevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

func relaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// Build argument string, quoting any arg that contains spaces.
	var parts []string
	for _, a := range os.Args[1:] {
		if strings.ContainsAny(a, ` \t`) {
			parts = append(parts, `"`+a+`"`)
		} else {
			parts = append(parts, a)
		}
	}
	argsStr := strings.Join(parts, " ")

	verb := syscall.StringToUTF16Ptr("runas")
	exeW := syscall.StringToUTF16Ptr(exe)
	var argsW *uint16
	if argsStr != "" {
		argsW = syscall.StringToUTF16Ptr(argsStr)
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	ret, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exeW)),
		uintptr(unsafe.Pointer(argsW)),
		0,
		1, // SW_NORMAL
	)
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW returned %d (elevation failed or cancelled)", ret)
	}

	os.Exit(0) // parent exits; elevated child takes over
	return nil
}
