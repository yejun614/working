package document

import (
	"errors"
	"strings"
	"testing"
)

// newSavedDocument는 본문을 채워 저장한 문서를 하나 만든다.
func newSavedDocument(t *testing.T, svc *Service, title, content string) string {
	t.Helper()
	doc, err := svc.Create(title, "", "markdown")
	if err != nil {
		t.Fatalf("Create 실패: %v", err)
	}
	doc.Content = content
	if _, err := svc.Save(doc); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}
	return doc.ID
}

func TestLockEncryptsStoredContent(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "사내 계정 정보입니다.")

	locked, err := svc.Lock(id, "열려라참깨", "참깨")
	if err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}
	if !locked.Locked || !locked.Unlocked {
		t.Fatalf("잠근 직후에는 잠긴 상태이면서 열려 있어야 합니다: %+v", locked)
	}
	// 방금 정한 암호이므로 본문은 그대로 편집을 이어갈 수 있어야 한다.
	if locked.Content != "사내 계정 정보입니다." {
		t.Fatalf("잠근 직후 본문 = %q", locked.Content)
	}

	// 저장소에는 평문이 남아 있어서는 안 된다.
	stored, err := svc.store.Get(id)
	if err != nil {
		t.Fatalf("저장소 조회 실패: %v", err)
	}
	if strings.Contains(stored.Content, "사내 계정") {
		t.Fatalf("저장소에 평문이 남아 있습니다: %q", stored.Content)
	}
	if !isLockedPayload(stored.Content) {
		t.Fatalf("저장소 본문이 잠긴 형식이 아닙니다: %q", stored.Content)
	}
}

func TestRelockHidesContent(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "숨길 내용")
	if _, err := svc.Lock(id, "암호1234", ""); err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}

	svc.Relock(id)

	// 다시 잠근 뒤에는 목록·조회 어디에도 본문이 실려 나가지 않아야 한다.
	docs, err := svc.List()
	if err != nil {
		t.Fatalf("List 실패: %v", err)
	}
	for _, doc := range docs {
		if doc.ID != id {
			continue
		}
		if doc.Content != "" || doc.Unlocked {
			t.Fatalf("잠긴 문서가 목록에 본문을 담아 보냈습니다: %+v", doc)
		}
	}
	got, err := svc.Get(id)
	if err != nil {
		t.Fatalf("Get 실패: %v", err)
	}
	if got.Content != "" {
		t.Fatalf("잠긴 문서의 본문 = %q, want 빈 문자열", got.Content)
	}
	if got.Hint != "" {
		t.Fatalf("힌트를 비워 두었는데 값이 있습니다: %q", got.Hint)
	}
}

func TestUnlockWithPassword(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "숨길 내용")
	if _, err := svc.Lock(id, "암호1234", "힌트입니다"); err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}
	svc.Relock(id)

	if _, err := svc.Unlock(id, "틀린암호"); !errors.Is(err, errWrongPassword) {
		t.Fatalf("틀린 암호 오류 = %v, want errWrongPassword", err)
	}

	opened, err := svc.Unlock(id, "암호1234")
	if err != nil {
		t.Fatalf("Unlock 실패: %v", err)
	}
	if opened.Content != "숨길 내용" || !opened.Unlocked {
		t.Fatalf("암호를 맞혔는데 본문이 열리지 않았습니다: %+v", opened)
	}
	if opened.Hint != "힌트입니다" {
		t.Fatalf("힌트 = %q, want %q", opened.Hint, "힌트입니다")
	}
}

func TestSaveLockedDocument(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "처음 내용")
	locked, err := svc.Lock(id, "암호1234", "")
	if err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}

	// 열어 둔 동안에는 평소처럼 저장되고, 저장 결과도 평문으로 돌아온다.
	locked.Content = "고친 내용"
	saved, err := svc.Save(locked)
	if err != nil {
		t.Fatalf("잠긴 문서 저장 실패: %v", err)
	}
	if saved.Content != "고친 내용" {
		t.Fatalf("저장 결과 본문 = %q, want %q", saved.Content, "고친 내용")
	}

	// 다시 잠그면 저장이 막히고, 암호를 넣으면 고친 내용이 그대로 나온다.
	svc.Relock(id)
	if _, err := svc.Save(saved); !errors.Is(err, errStillLocked) {
		t.Fatalf("잠긴 문서 저장 오류 = %v, want errStillLocked", err)
	}
	opened, err := svc.Unlock(id, "암호1234")
	if err != nil {
		t.Fatalf("Unlock 실패: %v", err)
	}
	if opened.Content != "고친 내용" {
		t.Fatalf("다시 열어 본 본문 = %q, want %q", opened.Content, "고친 내용")
	}
}

func TestRemoveLock(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "[[다른 문서]] 링크가 있는 내용")
	if _, err := svc.Lock(id, "암호1234", "힌트"); err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}
	svc.Relock(id)

	if _, err := svc.RemoveLock(id, "틀린암호"); !errors.Is(err, errWrongPassword) {
		t.Fatalf("틀린 암호 오류 = %v, want errWrongPassword", err)
	}

	opened, err := svc.RemoveLock(id, "암호1234")
	if err != nil {
		t.Fatalf("RemoveLock 실패: %v", err)
	}
	if opened.Locked || opened.Hint != "" {
		t.Fatalf("잠금이 남아 있습니다: %+v", opened)
	}
	// 평문으로 되돌렸으므로 링크 목록도 다시 계산되어야 한다.
	if len(opened.Links) != 1 || opened.Links[0] != "다른 문서" {
		t.Fatalf("링크 목록 = %v, want [다른 문서]", opened.Links)
	}
	stored, err := svc.store.Get(id)
	if err != nil {
		t.Fatalf("저장소 조회 실패: %v", err)
	}
	if isLockedPayload(stored.Content) {
		t.Fatalf("저장소에 아직 잠긴 본문이 있습니다: %q", stored.Content)
	}
}

func TestExportLockedDocumentFails(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "비밀", "숨길 내용")
	doc, err := svc.Lock(id, "암호1234", "")
	if err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}
	svc.Relock(id)

	if err := svc.Export(t.TempDir()+"/out.md", doc); !errors.Is(err, errStillLocked) {
		t.Fatalf("잠긴 문서 내보내기 오류 = %v, want errStillLocked", err)
	}
}

func TestReadOnlyBlocksSaveAndDelete(t *testing.T) {
	svc := tempDocumentService(t)
	id := newSavedDocument(t, svc, "완성한 문서", "그대로 두고 싶은 내용")

	doc, err := svc.SetReadOnly(id, true)
	if err != nil {
		t.Fatalf("SetReadOnly 실패: %v", err)
	}
	if !doc.ReadOnly {
		t.Fatal("읽기 전용으로 바뀌지 않았습니다")
	}

	doc.Content = "몰래 고친 내용"
	if _, err := svc.Save(doc); !errors.Is(err, errReadOnly) {
		t.Fatalf("읽기 전용 저장 오류 = %v, want errReadOnly", err)
	}
	if err := svc.Delete(id); !errors.Is(err, errReadOnly) {
		t.Fatalf("읽기 전용 삭제 오류 = %v, want errReadOnly", err)
	}

	// 읽기 전용을 끄면 다시 고칠 수 있어야 한다.
	if _, err := svc.SetReadOnly(id, false); err != nil {
		t.Fatalf("SetReadOnly 해제 실패: %v", err)
	}
	if _, err := svc.Save(doc); err != nil {
		t.Fatalf("읽기 전용을 끈 뒤 저장 실패: %v", err)
	}
	got, err := svc.Get(id)
	if err != nil {
		t.Fatalf("Get 실패: %v", err)
	}
	if got.Content != "몰래 고친 내용" {
		t.Fatalf("본문 = %q, want %q", got.Content, "몰래 고친 내용")
	}
}

func TestRenameKeepsLockedDocumentIntact(t *testing.T) {
	svc := tempDocumentService(t)
	lockedID := newSavedDocument(t, svc, "비밀", "[[예전 제목]]을 링크한 내용")
	targetID := newSavedDocument(t, svc, "예전 제목", "링크 대상")

	if _, err := svc.Lock(lockedID, "암호1234", ""); err != nil {
		t.Fatalf("Lock 실패: %v", err)
	}
	svc.Relock(lockedID)

	// 제목을 바꾸면 다른 문서의 링크를 함께 고치지만, 잠긴 문서는 건드릴 수 없다.
	target, err := svc.Get(targetID)
	if err != nil {
		t.Fatalf("Get 실패: %v", err)
	}
	target.Title = "새 제목"
	if _, err := svc.Save(target); err != nil {
		t.Fatalf("제목 변경 실패: %v", err)
	}

	opened, err := svc.Unlock(lockedID, "암호1234")
	if err != nil {
		t.Fatalf("Unlock 실패: %v", err)
	}
	if opened.Content != "[[예전 제목]]을 링크한 내용" {
		t.Fatalf("잠긴 문서의 본문이 바뀌었습니다: %q", opened.Content)
	}
}
