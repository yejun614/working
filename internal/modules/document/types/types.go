// Package types는 문서 모듈의 데이터 모델을 정의한다.
package types

// DocType은 문서를 어떤 편집기로 열지 정한다.
type DocType = string

const (
	// TypeMarkdown은 마크다운 원문을 편집하고 옆에서 미리 보는 형식이다.
	TypeMarkdown DocType = "markdown"

	// TypeRichText는 서식을 바로 보며 편집하는 WYSIWYG 형식이다.
	TypeRichText DocType = "wysiwyg"

	// TypePlainText는 서식 없이 원문만 다루는 형식이다.
	TypePlainText DocType = "text"
)

// NormalizeType은 알 수 없는 값이 들어오면 마크다운으로 되돌린다.
// 예전에 저장한 문서에는 타입이 없으므로 빈 값도 마크다운으로 본다.
func NormalizeType(value DocType) DocType {
	switch value {
	case TypeRichText, TypePlainText:
		return value
	default:
		return TypeMarkdown
	}
}

// Folder는 문서를 묶는 폴더이다.
// 폴더 안에 폴더를 둘 수 있고, 문서는 폴더 하나에만 속한다.
type Folder struct {
	// ID는 폴더 식별자(자동 생성)이다.
	ID string `json:"id"`

	// Name은 사이드바에 보이는 폴더 이름이다.
	Name string `json:"name"`

	// ParentID는 이 폴더를 담고 있는 상위 폴더이다. 비어 있으면 최상위이다.
	ParentID string `json:"parentId,omitempty"`

	// Order는 같은 상위 폴더 안에서의 정렬 순서다. 작을수록 위에 온다.
	// 순서를 한 번도 바꾸지 않은 폴더는 0이라 이름순으로 정렬된다.
	Order int `json:"order,omitempty"`

	// CreatedAt은 생성 시각(RFC3339)이다.
	CreatedAt string `json:"createdAt,omitempty"`
}

// Document는 문서 하나이다.
// 본문에 [[다른 문서 제목]] 형태로 링크를 적으면 문서 사이를 오갈 수 있다.
type Document struct {
	// ID는 문서 식별자(자동 생성). 제목이 바뀌어도 유지된다.
	ID string `json:"id"`

	// Title은 문서 제목이자 링크 대상 이름이다.
	// 링크가 제목으로 문서를 찾으므로 대소문자를 무시하고 유일해야 한다.
	Title string `json:"title"`

	// Type은 이 문서를 여는 편집기 형식이다. 문서마다 따로 정한다.
	Type DocType `json:"type,omitempty"`

	// FolderID는 문서가 담긴 폴더이다. 비어 있으면 폴더에 넣지 않은 문서다.
	FolderID string `json:"folderId,omitempty"`

	// Order는 같은 폴더 안에서의 정렬 순서다. 작을수록 위에 온다.
	// 순서를 한 번도 바꾸지 않은 문서는 0이라 최근 수정순으로 정렬된다.
	Order int `json:"order,omitempty"`

	// Content는 본문이다. 형식과 무관하게 마크다운 원문으로 보관한다.
	Content string `json:"content"`

	// Links는 본문이 참조하는 다른 문서 제목 목록이다.
	// 저장할 때 본문에서 다시 계산하므로 직접 채워 보낼 필요는 없다.
	Links []string `json:"links,omitempty"`

	// CreatedAt은 생성 시각(RFC3339).
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt은 마지막 수정 시각(RFC3339).
	UpdatedAt string `json:"updatedAt,omitempty"`
}
