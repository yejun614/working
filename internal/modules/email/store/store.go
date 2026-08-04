package store

import (
	"database/sql"

	"working/internal/modules/email/types"
	"working/internal/storage"
)

// cachedMessages는 폴더 하나의 메시지 캐시 스냅샷이다.
type cachedMessages struct {
	AccountID     string          `json:"accountId"`
	Folder        string          `json:"folder"`
	Messages      []types.Message `json:"messages"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	UpdatedAt     string          `json:"updatedAt"`
}

// Store는 이메일 모듈의 메시지 캐시를 보관한다.
// 계정과 키체인 자격증명은 통합 계정 모듈(internal/modules/account)이 관리한다.
type Store struct {
	db *sql.DB
}

func New() (*Store, error) {
	db, e := storage.Open()
	if e != nil {
		return nil, e
	}
	return &Store{db: db}, nil
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

// SetCachedUnread는 캐시된 메시지의 읽음 상태를 갱신한다.
// 메일 서버 반영이 끝난 뒤 호출해, 새로고침 없이도 목록 표시가 서버와 일치하게 한다.
func (s *Store) SetCachedUnread(acc, folder, id string, uid uint32, unread bool) error {
	page, found, e := s.CachedPage(acc, folder)
	if e != nil || !found {
		return e
	}
	changed := false
	for i := range page.Messages {
		if matchMessage(page.Messages[i], id, uid) && page.Messages[i].Unread != unread {
			page.Messages[i].Unread = unread
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.CachePage(acc, folder, page)
}

// RemoveCached는 삭제된 메시지를 캐시에서 제거한다.
func (s *Store) RemoveCached(acc, folder, id string, uid uint32) error {
	page, found, e := s.CachedPage(acc, folder)
	if e != nil || !found {
		return e
	}
	kept := make([]types.Message, 0, len(page.Messages))
	for _, m := range page.Messages {
		if !matchMessage(m, id, uid) {
			kept = append(kept, m)
		}
	}
	if len(kept) == len(page.Messages) {
		return nil
	}
	page.Messages = kept
	return s.CachePage(acc, folder, page)
}

// matchMessage는 원격 메시지 ID 또는 IMAP UID로 캐시된 메시지를 식별한다.
// Gmail은 ID를, IMAP은 UID를 사용하므로 둘 중 채워진 값으로 비교한다.
func matchMessage(m types.Message, id string, uid uint32) bool {
	if id != "" && m.ID != "" {
		return m.ID == id
	}
	return uid != 0 && m.UID == uid
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
