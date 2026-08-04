package document

import (
	"regexp"
	"strings"
)

// wikiLinkPattern은 본문에 적는 [[문서 제목]] 링크를 찾는다.
// 문서 제목은 한 줄이므로 대괄호와 줄바꿈은 링크 안에 올 수 없다.
var wikiLinkPattern = regexp.MustCompile(`\[\[([^\[\]\r\n]+)\]\]`)

// ParseLinks는 본문이 참조하는 문서 제목을 등장 순서대로 반환한다.
// 같은 문서를 여러 번 링크해도 한 번만 담고, 대소문자만 다른 제목은 같게 본다.
func ParseLinks(content string) []string {
	matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, match := range matches {
		title := strings.TrimSpace(match[1])
		if title == "" {
			continue
		}
		key := normalizeTitle(title)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, title)
	}
	return out
}

// LinksTo는 본문이 지정한 제목의 문서를 링크하는지 확인한다.
func LinksTo(content, title string) bool {
	target := normalizeTitle(title)
	if target == "" {
		return false
	}
	for _, link := range ParseLinks(content) {
		if normalizeTitle(link) == target {
			return true
		}
	}
	return false
}

// RenameLinks는 본문의 [[예전 제목]] 링크를 [[새 제목]]으로 바꾼다.
// 문서 제목을 바꿔도 다른 문서의 링크가 끊기지 않도록 저장 시 함께 호출한다.
func RenameLinks(content, oldTitle, newTitle string) string {
	target := normalizeTitle(oldTitle)
	if target == "" || target == normalizeTitle(newTitle) {
		return content
	}
	return wikiLinkPattern.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2]
		if normalizeTitle(inner) != target {
			return match
		}
		return "[[" + newTitle + "]]"
	})
}

// normalizeTitle은 제목 비교용 키를 만든다. 앞뒤 공백과 대소문자를 무시한다.
func normalizeTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}
