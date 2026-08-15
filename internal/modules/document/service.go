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

// defaultFolderName은 이름 없이 만든 폴더에 붙는 기본 이름이다.
const defaultFolderName = "새 폴더"

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
	// 타입을 도입하기 전에 만든 문서에는 값이 없으므로 마크다운으로 채워 보낸다.
	for i := range docs {
		docs[i].Type = types.NormalizeType(docs[i].Type)
	}
	return docs, nil
}

// Get은 ID로 문서를 조회한다.
func (s *Service) Get(id string) (*types.Document, error) {
	doc, err := s.store.Get(id)
	if err != nil || doc == nil {
		return doc, err
	}
	doc.Type = types.NormalizeType(doc.Type)
	return doc, nil
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
// folderID를 주면 그 폴더 안에 만들고, 비워 두면 폴더 밖에 만든다.
// 없는 문서로 향하는 링크를 눌렀을 때도 이 메서드로 문서를 만든다.
func (s *Service) Create(title string, folderID string, docType types.DocType) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	// 지워진 폴더의 ID가 남아 있으면 문서가 어디에도 보이지 않으므로 확인한다.
	folderID, err = s.existingFolderID(folderID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	doc := types.Document{
		ID:        newID(),
		Title:     uniqueTitle(docs, strings.TrimSpace(title), ""),
		Type:      types.NormalizeType(docType),
		FolderID:  folderID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Replace(append(docs, doc)); err != nil {
		return nil, err
	}
	return &doc, nil
}

// existingFolderID는 폴더가 실제로 있을 때만 그 ID를 돌려준다.
// 없는 폴더를 가리키면 폴더 밖(빈 문자열)으로 본다.
func (s *Service) existingFolderID(folderID string) (string, error) {
	if folderID == "" {
		return "", nil
	}
	folders, err := s.store.AllFolders()
	if err != nil {
		return "", err
	}
	for i := range folders {
		if folders[i].ID == folderID {
			return folderID, nil
		}
	}
	return "", nil
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

	folderID, err := s.existingFolderID(doc.FolderID)
	if err != nil {
		return nil, err
	}

	docs[index].Title = newTitle
	docs[index].Type = types.NormalizeType(doc.Type)
	docs[index].FolderID = folderID
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

// Folders는 이름순으로 모든 폴더를 반환한다.
func (s *Service) Folders() ([]types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	sort.Slice(folders, func(i, j int) bool {
		return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
	})
	return folders, nil
}

// CreateFolder는 새 폴더를 만든다. 같은 이름이 있으면 뒤에 번호를 붙인다.
func (s *Service) CreateFolder(name string) (*types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	folder := types.Folder{
		ID:        newID(),
		Name:      uniqueFolderName(folders, strings.TrimSpace(name), ""),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.store.ReplaceFolders(append(folders, folder)); err != nil {
		return nil, err
	}
	return &folder, nil
}

// RenameFolder는 폴더 이름을 바꾼다.
func (s *Service) RenameFolder(id string, name string) (*types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	for i := range folders {
		if folders[i].ID != id {
			continue
		}
		folders[i].Name = uniqueFolderName(folders, strings.TrimSpace(name), id)
		if err := s.store.ReplaceFolders(folders); err != nil {
			return nil, err
		}
		renamed := folders[i]
		return &renamed, nil
	}
	return nil, fmt.Errorf("폴더를 찾을 수 없습니다: %s", id)
}

// DeleteFolder는 폴더를 지운다.
// 안에 있던 문서는 함께 지우지 않고 폴더 밖으로 옮긴다.
func (s *Service) DeleteFolder(id string) error {
	folders, err := s.store.AllFolders()
	if err != nil {
		return err
	}
	kept := make([]types.Folder, 0, len(folders))
	for _, folder := range folders {
		if folder.ID != id {
			kept = append(kept, folder)
		}
	}
	if len(kept) == len(folders) {
		return fmt.Errorf("폴더를 찾을 수 없습니다: %s", id)
	}
	if err := s.store.ReplaceFolders(kept); err != nil {
		return err
	}

	docs, err := s.store.All()
	if err != nil {
		return err
	}
	moved := false
	for i := range docs {
		if docs[i].FolderID == id {
			docs[i].FolderID = ""
			moved = true
		}
	}
	if !moved {
		return nil
	}
	return s.store.Replace(docs)
}

// MoveDocument는 문서를 다른 폴더로 옮긴다. folderID가 비면 폴더 밖으로 뺀다.
// 옮기기는 내용 변경이 아니므로 마지막 수정 시각은 건드리지 않는다.
func (s *Service) MoveDocument(id string, folderID string) (*types.Document, error) {
	folderID, err := s.existingFolderID(folderID)
	if err != nil {
		return nil, err
	}
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	for i := range docs {
		if docs[i].ID != id {
			continue
		}
		if docs[i].FolderID == folderID {
			moved := docs[i]
			return &moved, nil
		}
		docs[i].FolderID = folderID
		if err := s.store.Replace(docs); err != nil {
			return nil, err
		}
		moved := docs[i]
		return &moved, nil
	}
	return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
}

// SetType은 문서를 여는 편집기 형식을 바꾼다.
// 본문은 그대로 두므로 마지막 수정 시각도 바꾸지 않는다.
func (s *Service) SetType(id string, docType types.DocType) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	for i := range docs {
		if docs[i].ID != id {
			continue
		}
		docs[i].Type = types.NormalizeType(docType)
		if err := s.store.Replace(docs); err != nil {
			return nil, err
		}
		changed := docs[i]
		return &changed, nil
	}
	return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
}

// uniqueFolderName은 겹치지 않는 폴더 이름을 만든다. 이미 있으면 " 2", " 3"을 붙인다.
func uniqueFolderName(folders []types.Folder, name, exceptID string) string {
	if name == "" {
		name = defaultFolderName
	}
	taken := func(candidate string) bool {
		target := normalizeTitle(candidate)
		for i := range folders {
			if folders[i].ID != exceptID && normalizeTitle(folders[i].Name) == target {
				return true
			}
		}
		return false
	}
	if !taken(name) {
		return name
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s %d", name, n)
		if !taken(candidate) {
			return candidate
		}
	}
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
