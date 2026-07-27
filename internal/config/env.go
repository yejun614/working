package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadEnv는 현재 작업 디렉터리의 .env를 읽어 환경변수로 등록한다.
// 운영 환경에서 이미 설정된 환경변수는 .env보다 우선한다.
// .env가 없어도 운영 환경변수만으로 실행할 수 있도록 오류로 처리하지 않는다.
func LoadEnv() error {
	path := filepath.Join(".env")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf(".env 파일 열기 실패: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) == "" {
			return fmt.Errorf(".env %d번째 줄 형식 오류: key=value가 필요합니다", lineNumber)
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '\'' && value[len(value)-1] == '\'') ||
				(value[0] == '"' && value[len(value)-1] == '"') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("환경변수 %q 설정 실패: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf(".env 읽기 실패: %w", err)
	}
	return nil
}

// GoogleClientID는 Google Calendar/Gmail OAuth 클라이언트 ID를 반환한다.
func GoogleClientID() string {
	return strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
}

// GoogleClientSecret은 Google Calendar/Gmail OAuth 클라이언트 보조 비밀값을 반환한다.
// 데스크톱 앱에 포함되는 값이므로 완전한 비밀로 간주하지 않지만, Google token 교환에는 필요하다.
func GoogleClientSecret() string {
	return strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
}
