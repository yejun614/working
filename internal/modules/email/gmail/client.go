package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"working/internal/config"
	"working/internal/modules/email/account"
	"working/internal/modules/email/imap"
	"working/internal/modules/email/smtp"
	"working/internal/modules/email/types"
)

const apiBase = "https://gmail.googleapis.com/gmail/v1/users/me"

// Client는 Gmail REST API를 호출하는 OAuth 인증 클라이언트이다.
type Client struct {
	httpClient *http.Client
}

// New는 키체인에 저장된 OAuth token JSON으로 Gmail 클라이언트를 만든다.
// 토큰이 갱신되면 save 콜백을 통해 키체인에 다시 저장한다.
func New(credential string, save func(string) error) (*Client, error) {
	if config.GoogleClientID() == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
	}
	var token oauth2.Token
	if err := json.Unmarshal([]byte(credential), &token); err != nil {
		return nil, fmt.Errorf("Gmail OAuth 토큰 파싱 실패: %w", err)
	}
	oauthConfig := &oauth2.Config{ClientID: config.GoogleClientID(), ClientSecret: config.GoogleClientSecret(), Endpoint: google.Endpoint}
	source := &savingTokenSource{
		source: oauthConfig.TokenSource(context.Background(), &token),
		save: func(updated *oauth2.Token) error {
			data, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			return save(string(data))
		},
	}
	return &Client{httpClient: oauth2.NewClient(context.Background(), source)}, nil
}

// Labels는 Gmail 라벨 이름 목록을 반환한다.
func (c *Client) Labels() ([]string, error) {
	labels, err := c.labels()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		names = append(names, label.Name)
	}
	return names, nil
}

// List는 Gmail 라벨의 최신 메시지를 최대 50개 조회한다.
func (c *Client) List(folder string) ([]types.Message, error) {
	page, err := c.ListPage(folder, "")
	return page.Messages, err
}

// ListPage는 Gmail 라벨의 메시지를 한 페이지 조회한다.
// Gmail이 발급한 pageToken을 그대로 다음 요청의 커서로 사용한다.
func (c *Client) ListPage(folder, pageToken string) (types.MessagePage, error) {
	if folder == "" {
		folder = "INBOX"
	}
	labelID, err := c.labelID(folder)
	if err != nil {
		return types.MessagePage{}, err
	}
	var result struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
		NextPageToken string `json:"nextPageToken"`
	}
	query := url.Values{}
	query.Set("labelIds", labelID)
	query.Set("maxResults", "50")
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	if err := c.get("/messages", query, &result); err != nil {
		return types.MessagePage{}, err
	}

	messages := make([]types.Message, 0, len(result.Messages))
	for _, item := range result.Messages {
		var detail struct {
			ID       string   `json:"id"`
			LabelIDs []string `json:"labelIds"`
			Raw      string   `json:"raw"`
		}
		params := url.Values{}
		params.Set("format", "raw")
		if err := c.get("/messages/"+url.PathEscape(item.ID), params, &detail); err != nil {
			return types.MessagePage{}, err
		}
		raw, err := decodeRawMessage(detail.Raw)
		if err != nil {
			return types.MessagePage{}, fmt.Errorf("Gmail 원문 디코딩 실패: %w", err)
		}
		message, err := imap.ParseRawMessage(raw)
		if err != nil {
			return types.MessagePage{}, fmt.Errorf("Gmail 원문 파싱 실패: %w", err)
		}
		message.UID = stableUID(detail.ID)
		message.Unread = contains(detail.LabelIDs, "UNREAD")
		messages = append(messages, message)
	}
	return types.MessagePage{Messages: messages, NextPageToken: result.NextPageToken}, nil
}

// decodeRawMessage는 Gmail API가 반환한 Base64 원문을 디코딩한다.
// Gmail은 Base64URL을 반환하는 것이 일반적이지만, 일부 응답이나 프록시 환경에서
// 표준 Base64가 전달될 수 있으므로 URL-safe/표준 형식과 패딩 유무를 모두 허용한다.
func decodeRawMessage(encoded string) ([]byte, error) {
	encoded = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, encoded)

	decoded, rawErr := base64.RawURLEncoding.DecodeString(encoded)
	if rawErr == nil {
		return decoded, nil
	}
	decoded, paddedErr := base64.URLEncoding.DecodeString(encoded)
	if paddedErr == nil {
		return decoded, nil
	}
	decoded, rawStdErr := base64.RawStdEncoding.DecodeString(encoded)
	if rawStdErr == nil {
		return decoded, nil
	}
	decoded, stdErr := base64.StdEncoding.DecodeString(encoded)
	if stdErr == nil {
		return decoded, nil
	}
	return nil, stdErr
}

type label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) labels() ([]label, error) {
	var result struct {
		Labels []label `json:"labels"`
	}
	if err := c.get("/labels", nil, &result); err != nil {
		return nil, err
	}
	return result.Labels, nil
}

func (c *Client) labelID(name string) (string, error) {
	labels, err := c.labels()
	if err != nil {
		return "", err
	}
	for _, label := range labels {
		if label.Name == name || label.ID == name {
			return label.ID, nil
		}
	}
	return name, nil
}

// Send는 Gmail API의 messages.send로 MIME 메일을 발송한다.
func (c *Client) Send(acc *account.Account, msg *types.Message) error {
	raw, err := smtp.BuildMIME(acc.Email, msg)
	if err != nil {
		return err
	}
	payload := struct {
		Raw string `json:"raw"`
	}{Raw: base64.RawURLEncoding.EncodeToString(raw)}
	var result struct {
		ID string `json:"id"`
	}
	return c.post("/messages/send", payload, &result)
}

func (c *Client) get(path string, query url.Values, out any) error {
	req, err := http.NewRequest(http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	return c.do(req, out)
}

func (c *Client) post(path string, payload any, out any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, apiBase+path, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Gmail API 요청 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("Gmail API 응답 오류(%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("Gmail API 응답 파싱 실패: %w", err)
	}
	return nil
}

func stableUID(id string) uint32 {
	if value, err := strconv.ParseUint(id, 16, 32); err == nil {
		return uint32(value)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type savingTokenSource struct {
	source oauth2.TokenSource
	save   func(*oauth2.Token) error
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if err := s.save(token); err != nil {
		return nil, fmt.Errorf("Gmail OAuth 토큰 저장 실패: %w", err)
	}
	return token, nil
}
