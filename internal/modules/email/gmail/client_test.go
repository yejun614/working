package gmail

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDecodeRawMessageAcceptsPaddedAndUnpaddedBase64URL(t *testing.T) {
	want := append([]byte("From: sender@example.com\r\n\r\nGmail message"), 0xfb, 0xff, 0xff)
	urlEncoded := base64.URLEncoding.EncodeToString(want)
	standardEncoded := base64.StdEncoding.EncodeToString(want)

	tests := []struct {
		name string
		data string
	}{
		{name: "URL 패딩 포함", data: urlEncoded},
		{name: "URL 패딩 없음", data: strings.TrimRight(urlEncoded, "=")},
		{name: "URL 줄바꿈 포함", data: urlEncoded[:8] + "\r\n" + urlEncoded[8:]},
		{name: "표준 Base64 패딩 포함", data: standardEncoded},
		{name: "표준 Base64 패딩 없음", data: strings.TrimRight(standardEncoded, "=")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeRawMessage(tt.data)
			if err != nil {
				t.Fatalf("Gmail 원문 디코딩 실패: %v", err)
			}
			if string(got) != string(want) {
				t.Fatalf("디코딩 결과가 다릅니다: got %q, want %q", got, want)
			}
		})
	}
}
