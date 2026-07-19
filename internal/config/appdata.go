package config

import (
	"os"
	"path/filepath"
)

// AppName은 앱 전체에서 사용하는 표시 이름이자 사용자 데이터 디렉토리 이름이다.
const AppName = "working"

// Dir은 사용자 데이터 디렉토리 경로를 반환한다.
// Windows: %LOCALAPPDATA%\working
// macOS:   ~/Library/Application Support/working
// Linux:   $XDG_DATA_HOME/working (없으면 ~/.local/share/working)
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, AppName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}