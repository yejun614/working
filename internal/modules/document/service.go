// Package document 는 "working" 앱의 문서 모듈 진입점이다.
//
// 마크다운 문서를 만들고 편집하며, 본문에 [[다른 문서 제목]]을 적어 문서
// 사이를 오갈 수 있다. 링크는 제목으로 대상을 찾으므로 제목은 대소문자를
// 무시하고 유일하게 유지된다. 제목을 바꾸면 다른 문서의 링크도 함께 갱신한다.
// 다른 모듈과 마찬가지로 main.go에서 Service를 등록하면 앱에 포함된다.
package document

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"working/internal/modules/document/store"
	"working/internal/modules/document/types"
)

// defaultTitle은 제목 없이 만든 문서에 붙는 기본 이름이다.
const defaultTitle = "제목 없는 문서"

// Service는 프론트엔드에 바인딩되는 문서 모듈 서비스이다.
type Service struct {
	store *store.Store
}

// NewService는 문서 모듈 Service를 생성한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
func (s *Service) ServiceShutdown() {}

// List는 최근 수정한 순서로 모든 문서를 반환한다.
// 프론트엔드가 링크 대상이 존재하는지 판단하려면 제목 전체가 필요하므로
// 목록에도 본문을 함께 담아 보낸다.
func (s *Service) List() ([]types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	sort.Slice(docs, func(i, j int) bool {
		if docs[i].UpdatedAt != docs[j].UpdatedAt {
			return docs[i].UpdatedAt > docs[j].UpdatedAt
		}
		return docs[i].Title < docs[j].Title
	})
	return docs, nil
}

// Get은 ID로 문서를 조회한다.
func (s *Service) Get(id string) (*types.Document, error) {
	return s.store.Get(id)
}

// FindByTitle은 제목으로 문서를 찾는다. 대소문자는 무시하며, 없으면 nil을 반환한다.
func (s *Service) FindByTitle(title string) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	target := normalizeTitle(title)
	for i := range docs {
		if normalizeTitle(docs[i].Title) == target {
			return &docs[i], nil
		}
	}
	return nil, nil
}

// Create는 새 문서를 만든다.
// 같은 제목이 이미 있으면 뒤에 번호를 붙여 유일한 제목으로 저장한다.
// 없는 문서로 향하는 링크를 눌렀을 때도 이 메서드로 문서를 만든다.
func (s *Service) Create(title string) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	doc := types.Document{
		ID:        newID(),
		Title:     uniqueTitle(docs, strings.TrimSpace(title), ""),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Replace(append(docs, doc)); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Save는 문서의 제목과 본문을 저장한다.
// 제목이 바뀌면 다른 문서의 [[예전 제목]] 링크도 새 제목으로 함께 고친다.
func (s *Service) Save(doc *types.Document) (*types.Document, error) {
	if doc == nil || doc.ID == "" {
		return nil, fmt.Errorf("문서 ID가 필요합니다")
	}
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	index := -1
	for i := range docs {
		if docs[i].ID == doc.ID {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", doc.ID)
	}

	oldTitle := docs[index].Title
	newTitle := strings.TrimSpace(doc.Title)
	if newTitle == "" {
		newTitle = oldTitle
	}
	if normalizeTitle(newTitle) != normalizeTitle(oldTitle) {
		if conflict := findByTitle(docs, newTitle, doc.ID); conflict != nil {
			return nil, fmt.Errorf("같은 제목의 문서가 이미 있습니다: %s", conflict.Title)
		}
	}

	docs[index].Title = newTitle
	docs[index].Content = doc.Content
	docs[index].Links = ParseLinks(doc.Content)
	docs[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// 제목이 바뀌었으면 다른 문서가 걸어 둔 링크가 끊기지 않게 함께 고친다.
	if normalizeTitle(newTitle) != normalizeTitle(oldTitle) {
		for i := range docs {
			if i == index {
				continue
			}
			updated := RenameLinks(docs[i].Content, oldTitle, newTitle)
			if updated == docs[i].Content {
				continue
			}
			docs[i].Content = updated
			docs[i].Links = ParseLinks(updated)
		}
	}

	if err := s.store.Replace(docs); err != nil {
		return nil, err
	}
	saved := docs[index]
	return &saved, nil
}

// Delete는 문서를 삭제한다.
// 다른 문서의 링크는 그대로 두어, 다시 만들면 링크가 되살아난다.
func (s *Service) Delete(id string) error {
	docs, err := s.store.All()
	if err != nil {
		return err
	}
	kept := make([]types.Document, 0, len(docs))
	for _, doc := range docs {
		if doc.ID != id {
			kept = append(kept, doc)
		}
	}
	if len(kept) == len(docs) {
		return fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
	}
	return s.store.Replace(kept)
}

// Backlinks는 이 문서를 링크하고 있는 문서를 최근 수정순으로 반환한다.
func (s *Service) Backlinks(id string) ([]types.Document, error) {
	docs, err := s.List()
	if err != nil {
		return nil, err
	}
	var target *types.Document
	for i := range docs {
		if docs[i].ID == id {
			target = &docs[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
	}
	out := make([]types.Document, 0)
	for _, doc := range docs {
		if doc.ID != id && LinksTo(doc.Content, target.Title) {
			out = append(out, doc)
		}
	}
	return out, nil
}

// findByTitle은 목록에서 제목이 같은 다른 문서를 찾는다.
func findByTitle(docs []types.Document, title, exceptID string) *types.Document {
	target := normalizeTitle(title)
	for i := range docs {
		if docs[i].ID != exceptID && normalizeTitle(docs[i].Title) == target {
			return &docs[i]
		}
	}
	return nil
}

// uniqueTitle은 겹치지 않는 제목을 만든다. 이미 있으면 " 2", " 3"을 붙인다.
func uniqueTitle(docs []types.Document, title, exceptID string) string {
	if title == "" {
		title = defaultTitle
	}
	if findByTitle(docs, title, exceptID) == nil {
		return title
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s %d", title, n)
		if findByTitle(docs, candidate, exceptID) == nil {
			return candidate
		}
	}
}

// newID는 16바이트 난수 기반의 문서 ID를 생성한다.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
