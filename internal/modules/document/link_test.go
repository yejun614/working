package document

import (
	"reflect"
	"testing"
)

func TestParseLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{name: "링크 없음", content: "그냥 본문입니다.", want: []string{}},
		{
			name:    "여러 링크를 등장 순서대로",
			content: "앞부분 [[회의록]] 중간 [[프로젝트 계획]] 끝",
			want:    []string{"회의록", "프로젝트 계획"},
		},
		{
			name:    "같은 문서를 여러 번 링크해도 한 번만",
			content: "[[회의록]]과 [[회의록]]",
			want:    []string{"회의록"},
		},
		{
			name:    "대소문자만 다르면 같은 문서로 본다",
			content: "[[Roadmap]] [[roadmap]]",
			want:    []string{"Roadmap"},
		},
		{name: "안쪽 공백은 제거", content: "[[  회의록  ]]", want: []string{"회의록"}},
		{name: "빈 링크는 무시", content: "[[]] [[   ]]", want: []string{}},
		{name: "대괄호 하나는 링크가 아님", content: "[회의록](http://example.com)", want: []string{}},
		{name: "여러 줄에 걸친 링크는 인식하지 않음", content: "[[회의\n록]]", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseLinks(tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseLinks = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLinksTo(t *testing.T) {
	content := "회의 내용은 [[프로젝트 계획]]을 참고하세요."

	if !LinksTo(content, "프로젝트 계획") {
		t.Fatal("링크된 문서를 찾지 못했습니다")
	}
	if !LinksTo(content, "  프로젝트 계획  ") {
		t.Fatal("앞뒤 공백이 있는 제목을 찾지 못했습니다")
	}
	if LinksTo(content, "회의록") {
		t.Fatal("링크하지 않은 문서를 찾았다고 보고했습니다")
	}
	if LinksTo(content, "") {
		t.Fatal("빈 제목은 링크로 보지 않아야 합니다")
	}
}

func TestRenameLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		oldTitle string
		newTitle string
		want     string
	}{
		{
			name:     "링크 제목을 바꾼다",
			content:  "[[회의록]]을 참고",
			oldTitle: "회의록",
			newTitle: "주간 회의록",
			want:     "[[주간 회의록]]을 참고",
		},
		{
			name:     "다른 링크는 건드리지 않는다",
			content:  "[[회의록]] [[프로젝트 계획]]",
			oldTitle: "회의록",
			newTitle: "주간 회의록",
			want:     "[[주간 회의록]] [[프로젝트 계획]]",
		},
		{
			name:     "대소문자와 공백이 달라도 바꾼다",
			content:  "[[ roadmap ]]",
			oldTitle: "Roadmap",
			newTitle: "제품 로드맵",
			want:     "[[제품 로드맵]]",
		},
		{
			name:     "제목이 그대로면 본문도 그대로",
			content:  "[[회의록]]",
			oldTitle: "회의록",
			newTitle: "회의록",
			want:     "[[회의록]]",
		},
		{
			name:     "본문에 같은 단어가 있어도 링크만 바꾼다",
			content:  "회의록 파일과 [[회의록]] 링크",
			oldTitle: "회의록",
			newTitle: "주간 회의록",
			want:     "회의록 파일과 [[주간 회의록]] 링크",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RenameLinks(tt.content, tt.oldTitle, tt.newTitle); got != tt.want {
				t.Fatalf("RenameLinks = %q, want %q", got, tt.want)
			}
		})
	}
}
