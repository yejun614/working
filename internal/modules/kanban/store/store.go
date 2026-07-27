package store

import (
	"database/sql"

	"working/internal/config"
	"working/internal/modules/kanban/types"
	"working/internal/storage"
)

const dataKey = "kanban.data"

type Data struct {
	Boards  []types.Board  `json:"boards"`
	Columns []types.Column `json:"columns"`
	Cards   []types.Card   `json:"cards"`
}
type Store struct{ db *sql.DB }

// New는 JSON 파일 대신 앱 공용 SQLite 저장소를 사용한다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	// 기존 설치의 칸반 JSON도 첫 실행 때 공용 SQLite로 이전한다.
	var legacy Data
	found, err := storage.GetJSON(db, dataKey, &legacy)
	if err != nil {
		return nil, err
	}
	if !found {
		dir, err := config.Dir()
		if err != nil {
			return nil, err
		}
		if ok, err := storage.LegacyJSON(dir, "kanban.json", &legacy); err != nil {
			return nil, err
		} else if ok {
			if err := storage.PutJSON(db, dataKey, &legacy); err != nil {
				return nil, err
			}
		}
	}
	return &Store{db: db}, nil
}
func (s *Store) Load() (*Data, error) {
	var d Data
	found, e := storage.GetJSON(s.db, dataKey, &d)
	if e != nil {
		return nil, e
	}
	if !found {
		return &Data{}, nil
	}
	return &d, nil
}
func (s *Store) Save(d *Data) error { return storage.PutJSON(s.db, dataKey, d) }
