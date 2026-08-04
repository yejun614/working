// Package store는 시계 모듈 설정을 앱 공용 SQLite에 보관한다.
package store

import (
	"database/sql"

	"working/internal/modules/clock/types"
	"working/internal/storage"
)

// settingsKey는 시계 설정을 저장하는 SQLite 키이다.
const settingsKey = "clock.settings"

// Store는 시계 설정 저장소이다.
type Store struct {
	db *sql.DB
}

// New는 앱 전체가 공유하는 SQLite 저장소를 사용하는 설정 저장소를 만든다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Load는 저장된 설정을 반환한다. 저장된 적이 없으면 found가 false다.
func (s *Store) Load() (types.Settings, bool, error) {
	var settings types.Settings
	found, err := storage.GetJSON(s.db, settingsKey, &settings)
	return settings, found, err
}

// Save는 설정을 저장한다.
func (s *Store) Save(settings types.Settings) error {
	return storage.PutJSON(s.db, settingsKey, settings)
}
