// Package types는 문서 모듈의 데이터 모델을 정의한다.
package types

// Document는 마크다운 문서 하나이다.
// 본문에 [[다른 문서 제목]] 형태로 링크를 적으면 문서 사이를 오갈 수 있다.
type Document struct {
	// ID는 문서 식별자(자동 생성). 제목이 바뀌어도 유지된다.
	ID string `json:"id"`

	// Title은 문서 제목이자 링크 대상 이름이다.
	// 링크가 제목으로 문서를 찾으므로 대소문자를 무시하고 유일해야 한다.
	Title string `json:"title"`

	// Content는 마크다운 본문이다.
	Content string `json:"content"`

	// Links는 본문이 참조하는 다른 문서 제목 목록이다.
	// 저장할 때 본문에서 다시 계산하므로 직접 채워 보낼 필요는 없다.
	Links []string `json:"links,omitempty"`

	// CreatedAt은 생성 시각(RFC3339).
	CreatedAt string `json:"createdAt,omitempty"`

	// UpdatedAt은 마지막 수정 시각(RFC3339).
	UpdatedAt string `json:"updatedAt,omitempty"`
}
