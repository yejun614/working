// Package googleoauth는 데스크톱 Google OAuth 인증 흐름을 제공한다.
package googleoauth

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Authenticate는 기본 브라우저와 loopback callback을 사용해 Google OAuth 토큰을 발급한다.
// PKCE verifier와 state를 사용하며, refresh token을 받을 수 있도록 offline access를 요청한다.
func Authenticate(clientID, clientSecret string, scopes []string) (*oauth2.Token, error) {
	if clientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET가 설정되지 않았습니다")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("OAuth callback 서버 시작 실패: %w", err)
	}
	defer listener.Close()

	state, err := randomURLValue(32)
	if err != nil {
		return nil, fmt.Errorf("OAuth state 생성 실패: %w", err)
	}
	verifier := oauth2.GenerateVerifier()
	oauthConfig := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://" + listener.Addr().String() + "/oauth2callback",
		Scopes:       scopes,
	}

	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("state") != state {
			resultCh <- callbackResult{err: fmt.Errorf("OAuth state 검증 실패")}
			_, _ = io.WriteString(w, "인증 요청이 유효하지 않습니다. 이 창을 닫아주세요.")
			return
		}
		if oauthErr := req.URL.Query().Get("error"); oauthErr != "" {
			resultCh <- callbackResult{err: fmt.Errorf("Google OAuth 인증 거부: %s", oauthErr)}
			_, _ = io.WriteString(w, "Google 인증이 취소되었습니다. 이 창을 닫아주세요.")
			return
		}
		code := req.URL.Query().Get("code")
		if code == "" {
			resultCh <- callbackResult{err: fmt.Errorf("OAuth authorization code가 없습니다")}
			_, _ = io.WriteString(w, "인증 코드가 없습니다. 이 창을 닫아주세요.")
			return
		}
		resultCh <- callbackResult{code: code}
		_, _ = io.WriteString(w, "Google 인증이 완료되었습니다. 이 창을 닫아주세요.")
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.S256ChallengeOption(verifier),
	)
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("Google 인증 브라우저 열기 실패: %w", err)
	}

	select {
	case callback := <-resultCh:
		if callback.err != nil {
			return nil, callback.err
		}
		token, err := oauthConfig.Exchange(context.Background(), callback.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("Google OAuth 토큰 교환 실패: %w", err)
		}
		return token, nil
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("Google OAuth 인증 시간이 초과되었습니다")
	}
}

func randomURLValue(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
