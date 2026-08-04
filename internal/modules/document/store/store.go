// Package store는 문서 목록을 앱 공용 SQLite에 보관한다.
package store

import (
	"database/sql"
	"fmt"

	"working/internal/modules/document/types"
	"working/internal/storage"
)

// documentsKey는 문서 목록을 저장하는 SQLite 키이다.
const documentsKey = "document.documents"

// Store는 문서 저장소이다.
type Store struct {
	db *sql.DB
}

// New는 앱 전체가 공유하는 SQLite 저장소를 사용하는 문서 저장소를 만든다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// All은 저장된 모든 문서를 반환한다.
func (s *Store) All() ([]types.Document, error) {
	var out []types.Document
	if _, err := storage.GetJSON(s.db, documentsKey, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []types.Document{}
	}
	return out, nil
}

// Get은 ID로 문서를 조회한다.
func (s *Store) Get(id string) (*types.Document, error) {
	docs, err := s.All()
	if err != nil {
		return nil, err
	}
	for i := range docs {
		if docs[i].ID == id {
			return &docs[i], nil
		}
	}
	return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
}

// Replace는 문서 목록 전체를 교체한다.
// 제목 변경으로 여러 문서의 링크를 한 번에 고쳐야 하므로 목록 단위로 저장한다.
func (s *Store) Replace(docs []types.Document) error {
	if docs == nil {
		docs = []types.Document{}
	}
	return storage.PutJSON(s.db, documentsKey, docs)
}
