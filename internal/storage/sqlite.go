// Package storage는 앱 전체가 공유하는 SQLite 데이터베이스를 제공한다.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
	"working/internal/config"
)

var (
	openMu     sync.Mutex
	shared     *sql.DB
	sharedPath string
)

// Open은 앱 데이터 디렉터리의 단일 SQLite 연결을 반환하고 기본 테이블을 만든다.
func Open() (*sql.DB, error) {
	openMu.Lock()
	defer openMu.Unlock()
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("SQLite 데이터 디렉터리 조회 실패: %w", err)
	}
	path := filepath.Join(dir, "working.sqlite3")
	if shared != nil && sharedPath == path {
		return shared, nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("SQLite 열기 실패: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS app_data (key TEXT PRIMARY KEY, value BLOB NOT NULL);`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("SQLite 초기화 실패: %w", err)
	}
	shared, sharedPath = db, path
	return db, nil
}

// Close는 공유 SQLite 연결을 닫는다.
// 다음 Open이 다시 연결하므로, 데이터 디렉터리를 바꾸거나 정리할 때 사용한다.
func Close() error {
	openMu.Lock()
	defer openMu.Unlock()
	if shared == nil {
		return nil
	}
	err := shared.Close()
	shared, sharedPath = nil, ""
	return err
}

// GetJSON은 키-값 저장소에서 JSON을 읽는다. 데이터가 없으면 found=false다.
func GetJSON(db *sql.DB, key string, out any) (found bool, err error) {
	var raw []byte
	err = db.QueryRow(`SELECT value FROM app_data WHERE key = ?`, key).Scan(&raw)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("SQLite 읽기 실패(%s): %w", key, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("SQLite JSON 파싱 실패(%s): %w", key, err)
	}
	return true, nil
}

// PutJSON은 값을 SQLite에 원자적으로 저장한다.
func PutJSON(db *sql.DB, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("SQLite JSON 직렬화 실패(%s): %w", key, err)
	}
	if _, err := db.Exec(`INSERT INTO app_data(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, raw); err != nil {
		return fmt.Errorf("SQLite 쓰기 실패(%s): %w", key, err)
	}
	return nil
}

// LegacyJSON은 기존 JSON 파일을 한 번만 읽어 SQLite로 이전할 수 있게 한다.
func LegacyJSON(dir, name string, out any) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return false, err
	}
	return true, nil
}
