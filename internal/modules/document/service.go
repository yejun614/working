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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"working/internal/modules/document/store"
	"working/internal/modules/document/types"
)

// defaultTitle은 제목 없이 만든 문서에 붙는 기본 이름이다.
const defaultTitle = "제목 없는 문서"

// defaultFolderName은 이름 없이 만든 폴더에 붙는 기본 이름이다.
const defaultFolderName = "새 폴더"

// errReadOnly는 읽기 전용 문서를 고치거나 지우려 할 때의 오류이다.
var errReadOnly = errors.New("읽기 전용 문서입니다. 수정하려면 읽기 전용을 먼저 끄세요")

// errStillLocked는 암호를 확인하지 않은 잠긴 문서를 다루려 할 때의 오류이다.
var errStillLocked = errors.New("잠긴 문서입니다. 암호를 먼저 입력하세요")

// Service는 프론트엔드에 바인딩되는 문서 모듈 서비스이다.
type Service struct {
	store *store.Store

	// mu는 아래 keys를 지킨다. 프론트엔드 호출이 동시에 들어올 수 있다.
	mu sync.Mutex

	// keys는 이번 실행에서 암호를 확인한 문서의 복호화 키이다.
	// 암호 자체는 어디에도 남기지 않으므로 앱을 다시 켜면 다시 물어본다.
	keys map[string]*lockKey
}

// NewService는 문서 모듈 Service를 생성한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st, keys: make(map[string]*lockKey)}, nil
}

// sessionKey는 이번 실행에서 열어 둔 문서의 키를 돌려준다. 없으면 nil이다.
func (s *Service) sessionKey(id string) *lockKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keys[id]
}

// rememberKey는 암호를 확인한 문서의 키를 앱이 켜져 있는 동안 기억한다.
func (s *Service) rememberKey(id string, key *lockKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[id] = key
}

// forgetKey는 기억해 둔 키를 지운다. 다시 잠글 때와 문서를 지울 때 쓴다.
func (s *Service) forgetKey(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.keys, id)
}

// present는 저장된 문서를 화면에 보낼 모습으로 다듬는다.
// 잠긴 문서는 본문이 암호화되어 있으므로, 이번 실행에서 암호를 확인한 문서만
// 본문을 풀어 담고 나머지는 본문을 비워 목록·검색·미리보기에 새지 않게 한다.
func (s *Service) present(doc *types.Document) {
	// 타입을 도입하기 전에 만든 문서에는 값이 없으므로 마크다운으로 채워 보낸다.
	doc.Type = types.NormalizeType(doc.Type)
	if !doc.Locked {
		return
	}
	if key := s.sessionKey(doc.ID); key != nil {
		if plain, err := key.open(doc.Content); err == nil {
			doc.Content = plain
			doc.Links = ParseLinks(plain)
			doc.Unlocked = true
			return
		}
	}
	doc.Content = ""
	doc.Links = nil
	doc.Unlocked = false
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
func (s *Service) ServiceShutdown() {}

// List는 사이드바에 보여 줄 순서대로 모든 문서를 반환한다.
// 순서를 바꾼 적이 없으면 Order가 모두 0이라 최근 수정순이 그대로 유지된다.
// 프론트엔드가 링크 대상이 존재하는지 판단하려면 제목 전체가 필요하므로
// 목록에도 본문을 함께 담아 보낸다.
func (s *Service) List() ([]types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	sortDocuments(docs)
	for i := range docs {
		s.present(&docs[i])
	}
	return docs, nil
}

// sortDocuments는 문서를 화면에 보이는 순서대로 정렬한다.
// 폴더별로 나누기 전에 전체를 한 번 정렬해 두면, 프론트엔드가 폴더로 걸러도
// 형제끼리의 순서가 그대로 유지된다.
func sortDocuments(docs []types.Document) {
	sort.SliceStable(docs, func(i, j int) bool {
		if docs[i].Order != docs[j].Order {
			return docs[i].Order < docs[j].Order
		}
		if docs[i].UpdatedAt != docs[j].UpdatedAt {
			return docs[i].UpdatedAt > docs[j].UpdatedAt
		}
		return docs[i].Title < docs[j].Title
	})
}

// sortFolders는 폴더를 화면에 보이는 순서대로 정렬한다.
func sortFolders(folders []types.Folder) {
	sort.SliceStable(folders, func(i, j int) bool {
		if folders[i].Order != folders[j].Order {
			return folders[i].Order < folders[j].Order
		}
		return strings.ToLower(folders[i].Name) < strings.ToLower(folders[j].Name)
	})
}

// Get은 ID로 문서를 조회한다.
func (s *Service) Get(id string) (*types.Document, error) {
	doc, err := s.store.Get(id)
	if err != nil || doc == nil {
		return doc, err
	}
	s.present(doc)
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
			s.present(&docs[i])
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
		ID:       newID(),
		Title:    uniqueTitle(docs, strings.TrimSpace(title), ""),
		Type:     types.NormalizeType(docType),
		FolderID: folderID,
		// 새 문서는 같은 폴더의 맨 위에 오도록 가장 작은 순서보다 하나 앞에 둔다.
		Order:     minDocumentOrder(docs, folderID) - 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.Replace(append(docs, doc)); err != nil {
		return nil, err
	}
	return &doc, nil
}

// minDocumentOrder는 폴더 안 문서 중 가장 작은 순서 값을 돌려준다.
func minDocumentOrder(docs []types.Document, folderID string) int {
	min := 0
	for i := range docs {
		if docs[i].FolderID == folderID && docs[i].Order < min {
			min = docs[i].Order
		}
	}
	return min
}

// importFileExtensions는 외부 파일을 가져올 때 본문으로 읽는 확장자이다.
// 그 밖의 파일도 읽을 수는 있으나, 문서로 만들기에 적합한 텍스트 계열만 기본 필터로 안내한다.
var importFileExtensions = []string{".md", ".markdown", ".txt"}

// Import는 외부 파일을 읽어 새 문서로 만든다.
// 제목은 파일 이름(확장자 제외)으로 하고, 본문은 파일 내용 그대로 담는다.
// folderID를 주면 그 폴더 안에 만들고, 비워 두면 폴더 밖에 만든다.
// 같은 제목이 이미 있으면 뒤에 번호를 붙여 유일한 제목으로 만든다.
func (s *Service) Import(path string, folderID string) (*types.Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("파일을 읽을 수 없습니다: %w", err)
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	// Create로 제목 충돌·폴더 검증·순서 처리를 재사용한 뒤,
	// 내용을 채워 Save로 저장하면 위키 링크도 함께 계산된다.
	doc, err := s.Create(title, folderID, types.TypeMarkdown)
	if err != nil {
		return nil, err
	}
	doc.Content = string(content)
	return s.Save(doc)
}

// Export는 문서 내용을 지정한 파일 경로에 쓴다.
// 문서는 존재해야 하며, 본문은 마크다운 원문 그대로 저장한다.
func (s *Service) Export(path string, doc *types.Document) error {
	if doc == nil || doc.ID == "" {
		return fmt.Errorf("문서 ID가 필요합니다")
	}
	found, err := s.Get(doc.ID)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("문서를 찾을 수 없습니다: %s", doc.ID)
	}
	// 잠긴 문서는 암호를 확인해야 본문이 채워진다. 빈 파일이 나가지 않게 막는다.
	if found.Locked && !found.Unlocked {
		return errStillLocked
	}
	return os.WriteFile(path, []byte(found.Content), 0o644)
}

// ImportPaths는 외부 파일 여러 개를 한꺼번에 가져와 새 문서로 만든다.
// 파일을 창에 끌어다 놓았을 때 백엔드가 받은 경로 목록을 처리하는 데 쓴다.
// 폴더 지정 없이 최상위로 만들고, 하나라도 실패하면 그 시점에 멈춘다.
func (s *Service) ImportPaths(paths []string) ([]types.Document, error) {
	out := make([]types.Document, 0, len(paths))
	for _, path := range paths {
		doc, err := s.Import(path, "")
		if err != nil {
			return out, err
		}
		out = append(out, *doc)
	}
	return out, nil
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
	// 읽기 전용 문서는 화면에서도 편집을 막지만, 자동 저장이 뒤늦게 도착할 수
	// 있으므로 저장 단계에서 한 번 더 막는다.
	if docs[index].ReadOnly {
		return nil, errReadOnly
	}
	// 잠긴 문서는 암호를 확인한 뒤에만 저장할 수 있다.
	var key *lockKey
	if docs[index].Locked {
		if key = s.sessionKey(doc.ID); key == nil {
			return nil, errStillLocked
		}
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
	if key != nil {
		// 잠긴 문서는 본문을 암호화해 넣고, 링크 목록도 남기지 않는다.
		// 링크 목록은 제목이라 그 자체로 내용을 짐작할 수 있기 때문이다.
		sealed, err := key.seal(doc.Content)
		if err != nil {
			return nil, err
		}
		docs[index].Content = sealed
		docs[index].Links = nil
	} else {
		docs[index].Content = doc.Content
		docs[index].Links = ParseLinks(doc.Content)
	}
	docs[index].UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// 제목이 바뀌었으면 다른 문서가 걸어 둔 링크가 끊기지 않게 함께 고친다.
	if normalizeTitle(newTitle) != normalizeTitle(oldTitle) {
		for i := range docs {
			if i == index {
				continue
			}
			// 잠긴 문서의 본문은 암호화되어 있어 링크를 고칠 수 없다.
			// 암호를 넣고 열면 예전 제목이 남아 있으므로 그때 직접 고쳐야 한다.
			if docs[i].Locked {
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
	s.present(&saved)
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
		if doc.ID == id {
			// 읽기 전용은 실수로 지우는 것까지 막는다.
			if doc.ReadOnly {
				return errReadOnly
			}
			continue
		}
		kept = append(kept, doc)
	}
	if len(kept) == len(docs) {
		return fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
	}
	if err := s.store.Replace(kept); err != nil {
		return err
	}
	s.forgetKey(id)
	return nil
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

// Folders는 사이드바에 보여 줄 순서대로 모든 폴더를 반환한다.
// 상위 폴더별로 나누기 전에 전체를 한 번 정렬해 두면, 프론트엔드가 상위
// 폴더로 걸러도 형제끼리의 순서가 그대로 유지된다.
func (s *Service) Folders() ([]types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	sortFolders(folders)
	return folders, nil
}

// CreateFolder는 새 폴더를 만든다. parentID를 주면 그 폴더 안에 만든다.
// 같은 상위 폴더에 이름이 겹치면 뒤에 번호를 붙인다.
func (s *Service) CreateFolder(name string, parentID string) (*types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	if !folderExists(folders, parentID) {
		parentID = ""
	}
	folder := types.Folder{
		ID:       newID(),
		Name:     uniqueFolderName(folders, strings.TrimSpace(name), parentID, ""),
		ParentID: parentID,
		// 새 폴더는 형제들 맨 아래에 붙인다.
		Order:     maxFolderOrder(folders, parentID) + 1,
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
		folders[i].Name = uniqueFolderName(folders, strings.TrimSpace(name), folders[i].ParentID, id)
		if err := s.store.ReplaceFolders(folders); err != nil {
			return nil, err
		}
		renamed := folders[i]
		return &renamed, nil
	}
	return nil, fmt.Errorf("폴더를 찾을 수 없습니다: %s", id)
}

// DeleteFolder는 폴더를 지운다.
// 안에 있던 하위 폴더와 문서는 함께 지우지 않고 한 단계 위로 끌어올린다.
func (s *Service) DeleteFolder(id string) error {
	folders, err := s.store.AllFolders()
	if err != nil {
		return err
	}
	parentID := ""
	found := false
	for i := range folders {
		if folders[i].ID == id {
			parentID = folders[i].ParentID
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("폴더를 찾을 수 없습니다: %s", id)
	}

	kept := make([]types.Folder, 0, len(folders))
	for _, folder := range folders {
		if folder.ID == id {
			continue
		}
		if folder.ParentID == id {
			folder.ParentID = parentID
		}
		kept = append(kept, folder)
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
			docs[i].FolderID = parentID
			moved = true
		}
	}
	if !moved {
		return nil
	}
	return s.store.Replace(docs)
}

// MoveDocument는 문서를 다른 폴더의 원하는 자리로 옮긴다.
// folderID가 비면 폴더 밖으로 빼고, index는 그 폴더 안에서 몇 번째에 둘지다.
// 옮기기는 내용 변경이 아니므로 마지막 수정 시각은 건드리지 않는다.
func (s *Service) MoveDocument(id string, folderID string, index int) (*types.Document, error) {
	folderID, err := s.existingFolderID(folderID)
	if err != nil {
		return nil, err
	}
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	sortDocuments(docs)

	moved := -1
	for i := range docs {
		if docs[i].ID == id {
			moved = i
			break
		}
	}
	if moved < 0 {
		return nil, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
	}
	docs[moved].FolderID = folderID

	// 옮긴 문서를 뺀 형제 목록에 원하는 자리로 다시 끼워 넣고 순서를 다시 매긴다.
	siblings := make([]int, 0, len(docs))
	for i := range docs {
		if i != moved && docs[i].FolderID == folderID {
			siblings = append(siblings, i)
		}
	}
	if index < 0 {
		index = 0
	}
	if index > len(siblings) {
		index = len(siblings)
	}
	ordered := make([]int, 0, len(siblings)+1)
	ordered = append(ordered, siblings[:index]...)
	ordered = append(ordered, moved)
	ordered = append(ordered, siblings[index:]...)
	for order, i := range ordered {
		docs[i].Order = order
	}

	if err := s.store.Replace(docs); err != nil {
		return nil, err
	}
	result := docs[moved]
	return &result, nil
}

// MoveFolder는 폴더를 다른 상위 폴더의 원하는 자리로 옮긴다.
// 자기 자신이나 자기 하위 폴더 안으로는 옮길 수 없다.
func (s *Service) MoveFolder(id string, parentID string, index int) (*types.Folder, error) {
	folders, err := s.store.AllFolders()
	if err != nil {
		return nil, err
	}
	sortFolders(folders)

	moved := -1
	for i := range folders {
		if folders[i].ID == id {
			moved = i
			break
		}
	}
	if moved < 0 {
		return nil, fmt.Errorf("폴더를 찾을 수 없습니다: %s", id)
	}
	if !folderExists(folders, parentID) {
		parentID = ""
	}
	// 자기 하위로 들어가면 트리가 끊겨 어디에도 보이지 않게 되므로 막는다.
	if parentID == id || isDescendantFolder(folders, parentID, id) {
		return nil, fmt.Errorf("폴더를 자기 자신이나 하위 폴더 안으로 옮길 수 없습니다")
	}

	previousParent := folders[moved].ParentID
	folders[moved].ParentID = parentID
	if previousParent != parentID {
		folders[moved].Name = uniqueFolderName(folders, folders[moved].Name, parentID, id)
	}

	siblings := make([]int, 0, len(folders))
	for i := range folders {
		if i != moved && folders[i].ParentID == parentID {
			siblings = append(siblings, i)
		}
	}
	if index < 0 {
		index = 0
	}
	if index > len(siblings) {
		index = len(siblings)
	}
	ordered := make([]int, 0, len(siblings)+1)
	ordered = append(ordered, siblings[:index]...)
	ordered = append(ordered, moved)
	ordered = append(ordered, siblings[index:]...)
	for order, i := range ordered {
		folders[i].Order = order
	}

	if err := s.store.ReplaceFolders(folders); err != nil {
		return nil, err
	}
	result := folders[moved]
	return &result, nil
}

// folderExists는 그 ID의 폴더가 있는지 알려준다. 빈 ID(최상위)는 항상 참이다.
func folderExists(folders []types.Folder, id string) bool {
	if id == "" {
		return true
	}
	for i := range folders {
		if folders[i].ID == id {
			return true
		}
	}
	return false
}

// isDescendantFolder는 candidate가 ancestor의 하위 폴더인지 확인한다.
func isDescendantFolder(folders []types.Folder, candidate, ancestor string) bool {
	parentOf := make(map[string]string, len(folders))
	for i := range folders {
		parentOf[folders[i].ID] = folders[i].ParentID
	}
	// 데이터가 어쩌다 순환하더라도 멈추도록 방문한 폴더를 기록한다.
	seen := make(map[string]bool, len(folders))
	for candidate != "" && !seen[candidate] {
		if candidate == ancestor {
			return true
		}
		seen[candidate] = true
		candidate = parentOf[candidate]
	}
	return false
}

// maxFolderOrder는 상위 폴더 안 폴더 중 가장 큰 순서 값을 돌려준다.
func maxFolderOrder(folders []types.Folder, parentID string) int {
	max := 0
	for i := range folders {
		if folders[i].ParentID == parentID && folders[i].Order > max {
			max = folders[i].Order
		}
	}
	return max
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

// findDocument는 목록에서 문서의 자리를 찾는다.
func findDocument(docs []types.Document, id string) (int, error) {
	for i := range docs {
		if docs[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("문서를 찾을 수 없습니다: %s", id)
}

// SetReadOnly는 문서를 읽기 전용(수정 금지)으로 바꾸거나 되돌린다.
// 읽기 전용인 동안에는 본문·제목을 저장할 수 없고 문서도 지울 수 없다.
// 본문을 건드리지 않으므로 마지막 수정 시각도 바꾸지 않는다.
func (s *Service) SetReadOnly(id string, readOnly bool) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	index, err := findDocument(docs, id)
	if err != nil {
		return nil, err
	}
	docs[index].ReadOnly = readOnly
	if err := s.store.Replace(docs); err != nil {
		return nil, err
	}
	changed := docs[index]
	s.present(&changed)
	return &changed, nil
}

// Lock은 문서를 암호로 잠근다.
// 본문은 암호에서 만든 키로 암호화해 보관하므로, 암호를 모르면 저장 파일을
// 직접 열어 보아도 내용을 알 수 없다. 암호는 어디에도 저장하지 않는다.
// hint는 암호를 잊었을 때 떠올릴 짧은 단서이며 비워 둘 수 있다.
// 방금 정한 암호이므로 이번 실행에서는 열어 둔 채로 편집을 이어갈 수 있다.
func (s *Service) Lock(id string, password string, hint string) (*types.Document, error) {
	if strings.TrimSpace(password) == "" {
		return nil, errors.New("암호를 입력하세요")
	}
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	index, err := findDocument(docs, id)
	if err != nil {
		return nil, err
	}
	if docs[index].Locked {
		return nil, errors.New("이미 암호로 잠긴 문서입니다")
	}
	key, err := newLockKey(password)
	if err != nil {
		return nil, err
	}
	sealed, err := key.seal(docs[index].Content)
	if err != nil {
		return nil, err
	}
	docs[index].Content = sealed
	docs[index].Locked = true
	docs[index].Hint = strings.TrimSpace(hint)
	// 링크 목록은 제목이라 그 자체로 내용을 짐작할 수 있으므로 남기지 않는다.
	docs[index].Links = nil
	if err := s.store.Replace(docs); err != nil {
		return nil, err
	}
	s.rememberKey(id, key)
	locked := docs[index]
	s.present(&locked)
	return &locked, nil
}

// Unlock은 암호를 확인해 잠긴 문서를 이번 실행 동안 열어 둔다.
// 성공하면 본문이 담긴 문서를 돌려주고, 이후 저장도 다시 가능해진다.
// 암호가 틀리면 그 사실만 알리는 오류를 돌려준다.
func (s *Service) Unlock(id string, password string) (*types.Document, error) {
	doc, err := s.store.Get(id)
	if err != nil {
		return nil, err
	}
	if !doc.Locked {
		s.present(doc)
		return doc, nil
	}
	key, plain, err := openLockKey(password, doc.Content)
	if err != nil {
		return nil, err
	}
	s.rememberKey(id, key)
	doc.Type = types.NormalizeType(doc.Type)
	doc.Content = plain
	doc.Links = ParseLinks(plain)
	doc.Unlocked = true
	return doc, nil
}

// Relock은 열어 둔 문서를 다시 잠근다. 암호는 그대로 남는다.
// 자리를 비울 때 눌러 두면 다시 볼 때 암호를 물어본다.
func (s *Service) Relock(id string) {
	s.forgetKey(id)
}

// RemoveLock은 암호를 확인한 뒤 잠금을 없애고 본문을 평문으로 되돌린다.
func (s *Service) RemoveLock(id string, password string) (*types.Document, error) {
	docs, err := s.store.All()
	if err != nil {
		return nil, err
	}
	index, err := findDocument(docs, id)
	if err != nil {
		return nil, err
	}
	if !docs[index].Locked {
		unlocked := docs[index]
		s.present(&unlocked)
		return &unlocked, nil
	}
	_, plain, err := openLockKey(password, docs[index].Content)
	if err != nil {
		return nil, err
	}
	docs[index].Content = plain
	docs[index].Locked = false
	docs[index].Hint = ""
	docs[index].Links = ParseLinks(plain)
	if err := s.store.Replace(docs); err != nil {
		return nil, err
	}
	s.forgetKey(id)
	opened := docs[index]
	s.present(&opened)
	return &opened, nil
}

// uniqueFolderName은 같은 상위 폴더 안에서 겹치지 않는 이름을 만든다.
// 이미 있으면 " 2", " 3"을 붙인다. 다른 폴더 안이라면 같은 이름을 써도 된다.
func uniqueFolderName(folders []types.Folder, name, parentID, exceptID string) string {
	if name == "" {
		name = defaultFolderName
	}
	taken := func(candidate string) bool {
		target := normalizeTitle(candidate)
		for i := range folders {
			if folders[i].ParentID != parentID {
				continue
			}
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
