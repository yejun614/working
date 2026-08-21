package document

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// 잠긴 문서의 본문 형식이다. 저장소에는 아래 모양의 한 줄로 들어간다.
//
//	doclock:1:<반복 횟수>:<소금>:<논스>:<암호문>
//
// 소금·논스·암호문은 base64이다. 반복 횟수를 함께 적어 두면 나중에 값을
// 올려도 예전에 잠근 문서를 그대로 열 수 있다.
const (
	lockPrefix    = "doclock:1:"
	lockFieldSize = 6
)

// lockIterations는 암호에서 키를 만들 때 쓰는 PBKDF2 반복 횟수이다.
// 사람이 기다릴 수 있는 시간(0.1초 안팎) 안에서 무차별 대입을 최대한 늦춘다.
const lockIterations = 210_000

// errWrongPassword는 암호가 맞지 않을 때의 오류이다.
// 프론트엔드가 "다시 입력" 안내를 보여 줄 수 있도록 다른 오류와 구분한다.
var errWrongPassword = errors.New("암호가 맞지 않습니다")

// lockKey는 문서 하나를 잠그고 여는 데 쓰는 키이다.
// 암호에서 키를 만드는 일은 일부러 느리게 만들어 두었으므로, 한 번 만든 키는
// 앱이 켜져 있는 동안 기억해 두고 저장할 때마다 다시 쓴다.
type lockKey struct {
	salt []byte
	iter int
	key  []byte
}

// newLockKey는 새 소금으로 암호에서 키를 만든다. 문서를 처음 잠글 때 쓴다.
func newLockKey(password string) (*lockKey, error) {
	if password == "" {
		return nil, errors.New("암호를 입력하세요")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("잠금 소금 생성 실패: %w", err)
	}
	return deriveLockKey(password, salt, lockIterations)
}

// openLockKey는 잠긴 본문에 적힌 소금으로 키를 만들어 본문을 풀어 본다.
// 암호가 틀리면 errWrongPassword를 돌려준다.
func openLockKey(password, payload string) (*lockKey, string, error) {
	salt, iter, nonce, ciphertext, err := parseLocked(payload)
	if err != nil {
		return nil, "", err
	}
	k, err := deriveLockKey(password, salt, iter)
	if err != nil {
		return nil, "", err
	}
	plain, err := k.decrypt(nonce, ciphertext)
	if err != nil {
		return nil, "", err
	}
	return k, plain, nil
}

// deriveLockKey는 암호와 소금으로 AES-256 키를 만든다.
func deriveLockKey(password string, salt []byte, iter int) (*lockKey, error) {
	key, err := pbkdf2.Key(sha256.New, password, salt, iter, 32)
	if err != nil {
		return nil, fmt.Errorf("잠금 키 생성 실패: %w", err)
	}
	return &lockKey{salt: salt, iter: iter, key: key}, nil
}

// seal은 본문을 암호화해 저장할 문자열로 만든다.
func (k *lockKey) seal(plaintext string) (string, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return "", fmt.Errorf("잠금 암호화 실패: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("잠금 암호화 실패: %w", err)
	}
	// 논스는 저장할 때마다 새로 뽑아야 한다. 같은 키로 같은 논스를 다시 쓰면
	// 두 본문을 비교해 내용을 추측할 수 있다.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("잠금 논스 생성 실패: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return lockPrefix + strings.Join([]string{
		strconv.Itoa(k.iter),
		base64.StdEncoding.EncodeToString(k.salt),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	}, ":"), nil
}

// open은 이미 만들어 둔 키로 잠긴 본문을 푼다.
func (k *lockKey) open(payload string) (string, error) {
	_, _, nonce, ciphertext, err := parseLocked(payload)
	if err != nil {
		return "", err
	}
	return k.decrypt(nonce, ciphertext)
}

// decrypt는 논스와 암호문을 본문으로 되돌린다.
// AES-GCM은 키가 틀리면 복호화 자체가 실패하므로 암호 확인도 여기서 끝난다.
func (k *lockKey) decrypt(nonce, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(k.key)
	if err != nil {
		return "", fmt.Errorf("잠금 복호화 실패: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("잠금 복호화 실패: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return "", errWrongPassword
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errWrongPassword
	}
	return string(plain), nil
}

// isLockedPayload는 저장된 본문이 잠긴 형식인지 알려준다.
func isLockedPayload(content string) bool {
	return strings.HasPrefix(content, lockPrefix)
}

// parseLocked는 저장된 잠금 문자열을 조각으로 나눈다.
func parseLocked(payload string) (salt []byte, iter int, nonce, ciphertext []byte, err error) {
	fields := strings.Split(payload, ":")
	if !isLockedPayload(payload) || len(fields) != lockFieldSize {
		return nil, 0, nil, nil, errors.New("잠긴 본문의 형식이 올바르지 않습니다")
	}
	iter, err = strconv.Atoi(fields[2])
	if err != nil || iter <= 0 {
		return nil, 0, nil, nil, errors.New("잠긴 본문의 반복 횟수가 올바르지 않습니다")
	}
	if salt, err = base64.StdEncoding.DecodeString(fields[3]); err != nil {
		return nil, 0, nil, nil, errors.New("잠긴 본문의 소금이 올바르지 않습니다")
	}
	if nonce, err = base64.StdEncoding.DecodeString(fields[4]); err != nil {
		return nil, 0, nil, nil, errors.New("잠긴 본문의 논스가 올바르지 않습니다")
	}
	if ciphertext, err = base64.StdEncoding.DecodeString(fields[5]); err != nil {
		return nil, 0, nil, nil, errors.New("잠긴 본문이 손상되었습니다")
	}
	return salt, iter, nonce, ciphertext, nil
}
