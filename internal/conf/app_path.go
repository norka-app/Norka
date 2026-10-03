package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppPathFileName is the executable path Norka writes next to config.toml.
// norka-cli reads it when it needs to start the app.
const AppPathFileName = "app-path"

// AppPathFile is the path file beside config.toml.
func AppPathFile(configPath string) string {
	return filepath.Join(filepath.Dir(strings.TrimSpace(configPath)), AppPathFileName)
}

// WriteAppPath records this process's executable next to config.toml.
// A later start overwrites the file, so an upgrade replaces a stale path.
func WriteAppPath(configPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil && resolved != "" {
		exe = resolved
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return err
	}
	return writeAppPath(configPath, abs)
}

// ReadAppPath returns the executable path recorded beside config.toml.
// The file must name an existing file. A missing file means Norka has not
// been started since this record was introduced.
func ReadAppPath(configPath string) (string, error) {
	data, err := os.ReadFile(AppPathFile(configPath))
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(data))
	if path == "" || strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("app path file is invalid")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("app path is not absolute")
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("saved Norka executable does not exist: %s", path)
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("saved Norka executable is a directory: %s", path)
	}
	return path, nil
}

func writeAppPath(configPath, exe string) error {
	path := AppPathFile(configPath)
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, PrivateDirPerm); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(exe+"\n"), PrivateFilePerm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
