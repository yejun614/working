package store

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/zalando/go-keyring"
	"working/internal/config"
	"working/internal/modules/email/account"
	"working/internal/modules/email/types"
	"working/internal/storage"
)

const accountsKey = "email.accounts"

type cachedMessages struct {
	AccountID     string          `json:"accountId"`
	Folder        string          `json:"folder"`
	Messages      []types.Message `json:"messages"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	UpdatedAt     string          `json:"updatedAt"`
}

type Store struct {
	db             *sql.DB
	keyringService string
}

func New() (*Store, error) {
	db, e := storage.Open()
	if e != nil {
		return nil, e
	}
	return &Store{db: db, keyringService: config.AppName + ".email"}, nil
}
func (s *Store) load() ([]account.Account, error) {
	var v []account.Account
	found, e := storage.GetJSON(s.db, accountsKey, &v)
	if e != nil {
		return nil, e
	}
	if !found {
		var old []account.Account
		if ok, x := storage.LegacyJSON(mustDir(), "email_accounts.json", &old); x != nil {
			return nil, x
		} else if ok {
			v = old
			if e = s.save(v); e != nil {
				return nil, e
			}
		}
	}
	if v == nil {
		v = []account.Account{}
	}
	return v, nil
}
func mustDir() string                           { d, _ := config.Dir(); return d }
func (s *Store) save(v []account.Account) error { return storage.PutJSON(s.db, accountsKey, v) }
func (s *Store) List() ([]account.Account, error) {
	v, e := s.load()
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v, e
}
func (s *Store) Get(id string) (*account.Account, error) {
	v, e := s.load()
	if e != nil {
		return nil, e
	}
	for i := range v {
		if v[i].ID == id {
			return &v[i], nil
		}
	}
	return nil, fmt.Errorf("계정을 찾을 수 없습니다: %s", id)
}
func (s *Store) Save(a *account.Account, c string) error {
	v, e := s.load()
	if e != nil {
		return e
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
	if c != "" {
		if e = keyring.Set(s.keyringService, a.ID, c); e != nil {
			return fmt.Errorf("키체인 저장 실패: %w", e)
		}
	}
	return s.save(v)
}
func (s *Store) Delete(id string) error {
	v, e := s.load()
	if e != nil {
		return e
	}
	kept := v[:0]
	for _, a := range v {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	if e = s.save(kept); e != nil {
		return e
	}
	_ = keyring.Delete(s.keyringService, id)
	return nil
}
func (s *Store) Credential(id string) (string, error) {
	v, e := keyring.Get(s.keyringService, id)
	if e == keyring.ErrNotFound {
		return "", fmt.Errorf("자격증명이 키체인에 없습니다: %s", id)
	}
	if e != nil {
		return "", fmt.Errorf("키체인 조회 실패: %w", e)
	}
	return v, nil
}
func cacheKey(acc, folder string) string { return "email.messages." + acc + "." + folder }

// Cached는 네트워크를 호출하지 않고 SQLite에 저장된 목록과 본문을 반환한다.
func (s *Store) Cached(acc, folder string) ([]types.Message, bool, error) {
	page, found, err := s.CachedPage(acc, folder)
	return page.Messages, found, err
}

// CachedPage는 캐시된 메시지와 다음 서버 페이지 커서를 반환한다.
func (s *Store) CachedPage(acc, folder string) (types.MessagePage, bool, error) {
	var c cachedMessages
	found, err := storage.GetJSON(s.db, cacheKey(acc, folder), &c)
	return types.MessagePage{Messages: c.Messages, NextPageToken: c.NextPageToken}, found, err
}

// Cache는 목록과 이메일 본문/원문을 SQLite에 저장한다.
func (s *Store) Cache(acc, folder string, msgs []types.Message) error {
	return s.CachePage(acc, folder, types.MessagePage{Messages: msgs})
}

// CachePage는 목록과 다음 서버 페이지 커서를 SQLite에 저장한다.
func (s *Store) CachePage(acc, folder string, page types.MessagePage) error {
	return storage.PutJSON(s.db, cacheKey(acc, folder), cachedMessages{AccountID: acc, Folder: folder, Messages: page.Messages, NextPageToken: page.NextPageToken})
}

// CachedFolders는 SQLite에 기록된 폴더만 반환한다.
func (s *Store) CachedFolders(acc string) ([]string, error) {
	var keys []string
	rows, e := s.db.Query(`SELECT key FROM app_data WHERE key LIKE ?`, "email.messages."+acc+".%")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		if e = rows.Scan(&k); e != nil {
			return nil, e
		}
		keys = append(keys, k[len("email.messages."+acc+"."):])
	}
	return keys, nil
}

// CacheJSON는 동기화 결과의 부가 메타데이터를 저장한다.
func (s *Store) CacheJSON(key string, v any) error { return storage.PutJSON(s.db, "email."+key, v) }
func (s *Store) RawCache(key string, out any) (bool, error) {
	return storage.GetJSON(s.db, "email."+key, out)
}
