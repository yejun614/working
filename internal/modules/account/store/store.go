// Package store는 통합 계정 목록과 키체인 자격증명을 관리한다.
package store

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/zalando/go-keyring"

	"working/internal/config"
	"working/internal/modules/account/types"
	"working/internal/storage"
)

// accountsKey는 통합 계정 목록을 저장하는 SQLite 키이다.
const accountsKey = "account.accounts"

// Store는 통합 계정 저장소이다.
type Store struct {
	db             *sql.DB
	keyringService string
}

// New는 앱 전체가 공유하는 SQLite 저장소를 사용하는 계정 저장소를 만든다.
// 통합 계정 목록이 아직 없으면 기존 모듈별 계정을 한 번 이관한다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, keyringService: config.AppName + ".account"}
	if err := s.migrateLegacyAccounts(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() ([]types.Account, error) {
	var out []types.Account
	if _, err := storage.GetJSON(s.db, accountsKey, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []types.Account{}
	}
	return out, nil
}

func (s *Store) save(v []types.Account) error { return storage.PutJSON(s.db, accountsKey, v) }

// List는 등록된 모든 계정을 반환한다(자격증명 제외).
func (s *Store) List() ([]types.Account, error) {
	v, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v, nil
}

// Get은 ID로 계정을 조회한다.
func (s *Store) Get(id string) (*types.Account, error) {
	v, err := s.load()
	if err != nil {
		return nil, err
	}
	for i := range v {
		if v[i].ID == id {
			return &v[i], nil
		}
	}
	return nil, fmt.Errorf("계정을 찾을 수 없습니다: %s", id)
}

// Save는 계정을 등록하거나 갱신한다.
// credential이 빈 문자열이면 키체인의 기존 자격증명을 그대로 둔다.
func (s *Store) Save(a *types.Account, credential string) error {
	v, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i := range v {
		if v[i].ID == a.ID {
			v[i] = *a
			found = true
		}
	}
	if !found {
		v = append(v, *a)
	}
	if credential != "" {
		if err := keyring.Set(s.keyringService, a.ID, credential); err != nil {
			return fmt.Errorf("키체인 저장 실패: %w", err)
		}
	}
	return s.save(v)
}

// Delete는 계정과 키체인 자격증명을 함께 삭제한다.
func (s *Store) Delete(id string) error {
	v, err := s.load()
	if err != nil {
		return err
	}
	kept := make([]types.Account, 0, len(v))
	for _, a := range v {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	if err := s.save(kept); err != nil {
		return err
	}
	_ = keyring.Delete(s.keyringService, id)
	return nil
}

// Credential은 키체인에서 계정 자격증명을 읽는다.
// OAuth 계정은 이 값 하나를 메일과 캘린더가 함께 사용한다.
func (s *Store) Credential(id string) (string, error) {
	v, err := keyring.Get(s.keyringService, id)
	if err == keyring.ErrNotFound {
		return "", fmt.Errorf("자격증명이 키체인에 없습니다: %s", id)
	}
	if err != nil {
		return "", fmt.Errorf("키체인 조회 실패: %w", err)
	}
	return v, nil
}

// SaveCredential은 계정 메타데이터를 건드리지 않고 자격증명만 교체한다.
// OAuth access token이 갱신되었을 때 사용한다.
func (s *Store) SaveCredential(id, credential string) error {
	if credential == "" {
		return nil
	}
	if err := keyring.Set(s.keyringService, id, credential); err != nil {
		return fmt.Errorf("키체인 저장 실패: %w", err)
	}
	return nil
}

// SetAuthError는 재인증 안내 상태만 갱신한다.
// 값이 바뀌지 않았으면 저장하지 않는다.
func (s *Store) SetAuthError(id, message string) error {
	acc, err := s.Get(id)
	if err != nil {
		return err
	}
	if acc.AuthError == message {
		return nil
	}
	acc.AuthError = message
	return s.Save(acc, "")
}
