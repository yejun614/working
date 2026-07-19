package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"working/internal/config"
	"working/internal/modules/email/account"

	"github.com/zalando/go-keyring"
)

// accountsFile은 계정 메타데이터 파일명이다.
const accountsFile = "email_accounts.json"

// Store는 이메일 계정 메타데이터와 자격증명을 관리한다.
// 계정 메타데이터(이름, 서버 정보 등)는 사용자 데이터 디렉토리의
// JSON 파일에 저장되고, 비밀번호/토큰은 OS 키체인에 저장된다.
type Store struct {
	// dataDir은 메타데이터 파일이 위치하는 디렉토리.
	dataDir string

	// keyringService는 키체인 서비스 식별자.
	keyringService string
}

// New는 기본 사용자 데이터 디렉토리를 사용하는 Store를 생성한다.
func New() (*Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("사용자 데이터 디렉토리 조회 실패: %w", err)
	}
	return &Store{
		dataDir:        dir,
		keyringService: config.AppName + ".email",
	}, nil
}

// List는 등록된 모든 계정을 ID순으로 반환한다.
// 비밀번호는 절대 포함되지 않는다.
func (s *Store) List() ([]account.Account, error) {
	accs, err := s.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(accs, func(i, j int) bool { return accs[i].ID < accs[j].ID })
	return accs, nil
}

// Get은 ID로 계정을 조회한다.
func (s *Store) Get(id string) (*account.Account, error) {
	accs, err := s.load()
	if err != nil {
		return nil, err
	}
	for i := range accs {
		if accs[i].ID == id {
			return &accs[i], nil
		}
	}
	return nil, fmt.Errorf("계정을 찾을 수 없습니다: %s", id)
}

// Save는 계정 메타데이터를 저장하고, credential이 비어있지 않으면
// 키체인에도 자격증명을 저장한다. credential이 빈 문자열이면
// 기존 키체인 항목을 그대로 유지한다.
func (s *Store) Save(acc *account.Account, credential string) error {
	accs, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i := range accs {
		if accs[i].ID == acc.ID {
			accs[i] = *acc
			found = true
			break
		}
	}
	if !found {
		accs = append(accs, *acc)
	}

	if credential != "" {
		if err := keyring.Set(s.keyringService, acc.ID, credential); err != nil {
			return fmt.Errorf("키체인 저장 실패: %w", err)
		}
	}
	return s.save(accs)
}

// Delete는 계정 메타데이터와 키체인 자격증명을 함께 삭제한다.
// 메타데이터가 없더라도 키체인 항목은 시도한다(누적 잔여 제거).
func (s *Store) Delete(id string) error {
	accs, err := s.load()
	if err != nil {
		return err
	}
	kept := accs[:0]
	for _, a := range accs {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	if err := s.save(kept); err != nil {
		return err
	}
	// 키체인 항목 삭제: 존재하지 않아도 에러는 무시.
	_ = keyring.Delete(s.keyringService, id)
	return nil
}

// Credential은 계정의 자격증명(비밀번호/토큰)을 키체인에서 조회한다.
func (s *Store) Credential(id string) (string, error) {
	cred, err := keyring.Get(s.keyringService, id)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("자격증명이 키체인에 없습니다: %s", id)
		}
		return "", fmt.Errorf("키체인 조회 실패: %w", err)
	}
	return cred, nil
}

// load는 메타데이터 파일을 읽어 계정 목록을 반환한다.
// 파일이 없으면 빈 목록을 반환한다.
func (s *Store) load() ([]account.Account, error) {
	path := filepath.Join(s.dataDir, accountsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []account.Account{}, nil
		}
		return nil, fmt.Errorf("계정 파일 읽기 실패: %w", err)
	}
	var accs []account.Account
	if err := json.Unmarshal(data, &accs); err != nil {
		return nil, fmt.Errorf("계정 파일 파싱 실패: %w", err)
	}
	if accs == nil {
		accs = []account.Account{}
	}
	return accs, nil
}

// save는 계정 목록을 메타데이터 파일에 쓴다.
func (s *Store) save(accs []account.Account) error {
	path := filepath.Join(s.dataDir, accountsFile)
	data, err := json.MarshalIndent(accs, "", "  ")
	if err != nil {
		return fmt.Errorf("계정 직렬화 실패: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("계정 파일 쓰기 실패: %w", err)
	}
	return nil
}