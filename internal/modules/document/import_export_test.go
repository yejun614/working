package document

import (
	"os"
	"path/filepath"
	"testing"

	"working/internal/modules/document/types"
	"working/internal/storage"
)

// tempDocumentService는 테스트용 임시 서비스와 저장소를 만든다.
// 문서 모듈의 저장소는 앱 공용 SQLite를 쓰므로, 테스트 간에 데이터가 섞이지 않도록
// 임시 데이터 디렉터리로 바꿔 준 뒤 서비스를 생성한다.
func tempDocumentService(t *testing.T) *Service {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Cleanup(func() { _ = storage.Close() })
	svc, err := NewService()
	if err != nil {
		t.Fatalf("서비스 생성 실패: %v", err)
	}
	return svc
}

func TestImport(t *testing.T) {
	svc := tempDocumentService(t)

	// 임시 파일을 만들어 가져온다. 제목은 파일명(확장자 제외), 본문은 파일 내용이어야 한다.
	src := filepath.Join(t.TempDir(), "회의록.md")
	if err := os.WriteFile(src, []byte("# 회의록\n\n내용입니다."), 0o644); err != nil {
		t.Fatalf("임시 파일 작성 실패: %v", err)
	}

	doc, err := svc.Import(src, "")
	if err != nil {
		t.Fatalf("Import 실패: %v", err)
	}
	if doc.Title != "회의록" {
		t.Fatalf("제목 = %q, want %q", doc.Title, "회의록")
	}
	if doc.Content != "# 회의록\n\n내용입니다." {
		t.Fatalf("본문이 파일 내용과 다릅니다: %q", doc.Content)
	}

	// 같은 이름의 파일을 다시 가져오면 제목에 번호가 붙어 유일해진다.
	doc2, err := svc.Import(src, "")
	if err != nil {
		t.Fatalf("두 번째 Import 실패: %v", err)
	}
	if doc2.Title == doc.Title {
		t.Fatalf("제목이 겹치면 안 됩니다: %q", doc2.Title)
	}
}

func TestImportMissingFile(t *testing.T) {
	svc := tempDocumentService(t)
	if _, err := svc.Import(filepath.Join(t.TempDir(), "없는파일.md"), ""); err == nil {
		t.Fatal("없는 파일을 가져왔는데 오류가 나지 않았습니다")
	}
}

func TestExport(t *testing.T) {
	svc := tempDocumentService(t)

	// 먼저 문서를 하나 만들어 저장소에 넣는다.
	doc, err := svc.Create("회의록", "", "markdown")
	if err != nil {
		t.Fatalf("Create 실패: %v", err)
	}
	doc.Content = "회의 내용입니다."
	if _, err := svc.Save(doc); err != nil {
		t.Fatalf("Save 실패: %v", err)
	}

	// 저장된 문서를 파일로 내보낸 뒤 내용이 같은지 확인한다.
	dest := filepath.Join(t.TempDir(), "export.md")
	if err := svc.Export(dest, doc); err != nil {
		t.Fatalf("Export 실패: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("내보낸 파일 읽기 실패: %v", err)
	}
	if string(got) != "회의 내용입니다." {
		t.Fatalf("내보낸 내용 = %q, want %q", string(got), "회의 내용입니다.")
	}
}

func TestExportMissingDocument(t *testing.T) {
	svc := tempDocumentService(t)
	dest := filepath.Join(t.TempDir(), "out.md")
	if err := svc.Export(dest, &types.Document{ID: "없는-id"}); err == nil {
		t.Fatal("없는 문서를 내보냈는데 오류가 나지 않았습니다")
	}
}

func TestImportPaths(t *testing.T) {
	svc := tempDocumentService(t)

	dir := t.TempDir()
	one := filepath.Join(dir, "하나.md")
	two := filepath.Join(dir, "둘.txt")
	if err := os.WriteFile(one, []byte("하나"), 0o644); err != nil {
		t.Fatalf("임시 파일 작성 실패: %v", err)
	}
	if err := os.WriteFile(two, []byte("둘"), 0o644); err != nil {
		t.Fatalf("임시 파일 작성 실패: %v", err)
	}

	docs, err := svc.ImportPaths([]string{one, two})
	if err != nil {
		t.Fatalf("ImportPaths 실패: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("가져온 문서 수 = %d, want 2", len(docs))
	}
	if docs[0].Title != "하나" || docs[1].Title != "둘" {
		t.Fatalf("문서 제목이 예상과 다릅니다: %q, %q", docs[0].Title, docs[1].Title)
	}
}
