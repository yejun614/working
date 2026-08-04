package caldav

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"golang.org/x/oauth2"
)

func TestIsAuthError(t *testing.T) {
	// 실제로 관측된 오류 형태: oauth2 토큰 갱신 실패가 http.Client.Do에서 *url.Error로 감싸진다.
	expiredToken := &url.Error{
		Op:  "Propfind",
		URL: "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user",
		Err: &oauth2.RetrieveError{
			ErrorCode:        "invalid_grant",
			ErrorDescription: "Token has been expired or revoked.",
		},
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil은 인증 오류가 아님", err: nil, want: false},
		{name: "url.Error로 감싸인 invalid_grant", err: expiredToken, want: true},
		{name: "한 번 더 감싸도 판별", err: fmt.Errorf("동기화 실패: %w", expiredToken), want: true},
		{
			name: "401 응답은 재인증 대상",
			err:  &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusUnauthorized}, Body: []byte("unauthorized")},
			want: true,
		},
		{
			name: "일시적인 서버 오류는 재인증 대상이 아님",
			err:  &oauth2.RetrieveError{Response: &http.Response{StatusCode: http.StatusInternalServerError}, Body: []byte("boom")},
			want: false,
		},
		{name: "일반 네트워크 오류", err: errors.New("dial tcp: connection refused"), want: false},
		{name: "CalDAV 상태 코드 오류", err: fmt.Errorf("일정 조회 실패: HTTP 507"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.want {
				t.Fatalf("IsAuthError = %v, want %v", got, tt.want)
			}
		})
	}
}
