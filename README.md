# working

모듈 단위로 기능을 확장할 수 있는 업무 보조 프로그램. 각 기능은
`internal/modules/<이름>` 패키지로 격리되어 있고, `main.go`에서 원하는
모듈의 Wails Service만 `application.NewService(...)`로 등록하면 해당
기능만 앱에 포함된다.

## 기술 스택

- Go 1.25+ / [Wails v3](https://v3.wails.io/) (데스크톱 앱 프레임워크)
- Vue 3 + TypeScript + Vite (프론트엔드)
- OS 키체인([github.com/zalando/go-keyring](https://github.com/zalando/go-keyring))

## 디렉토리 구조

```
working/
├─ main.go                          # 앱 진입점. 사용할 모듈 Service를 등록
├─ internal/
│  ├─ config/                       # 사용자 데이터 디렉토리 등 공용 설정
│  └─ modules/
│     └─ email/                     # 이메일 모듈
│        ├─ service.go              # Wails Service (프론트엔드 API 진입점)
│        ├─ types/types.go          # Message, Attachment
│        ├─ account/account.go      # Account, ServerConfig 타입
│        ├─ store/store.go          # 계정 메타데이터 + 키체인 자격증명
│        ├─ smtp/                   # SMTP 발송 (none/STARTTLS/TLS)
│        └─ imap/                   # IMAP 수신 (폴더 목록, 메시지 조회)
├─ frontend/
│  ├─ src/components/email/         # 이메일 UI (목록/상세/계정관리/작성)
│  └─ bindings/                     # Wails가 자동 생성하는 TS 바인딩
└─ build/                           # 플랫폼별 빌드/패키지 설정
```

## 이메일 모듈

### 기능

- **계정 관리**: 복수 계정 등록/편집/삭제. 비밀번호·토큰은 OS 키체인에
  저장되고 메타데이터(이름, 서버 정보 등)는 사용자 데이터 디렉토리의
  JSON 파일에 저장된다. 계정 삭제 시 키체인 항목도 함께 제거된다.
- **메일 전송 (SMTP)**: `none` / `STARTTLS` / `TLS(암시적)` 지원.
  첨부파일 MIME 인코딩, RFC 2047 제목/표시명 인코딩 포함.
- **메일 수신 (IMAP)**: 폴더 목록 조회, 폴더별 최신 메시지 50건 조회.
  multipart/alternative 본문 추출(HTML → 텍스트 변환).

### 프론트엔드 API

`frontend/bindings/working/internal/modules/email/service.ts`에서
자동 생성된 바인딩을 통해 아래 메서드를 호출한다.

| 메서드              | 설명                                                |
| ------------------- | --------------------------------------------------- |
| `AccountList()`     | 등록된 계정 목록                                    |
| `AccountGet(id)`    | ID로 계정 조회                                      |
| `AccountCreate(acc, credential)` | 신규 계정 등록 (자격증명은 키체인 저장) |
| `AccountUpdate(acc, credential)` | 계정 수정 (credential 빈 값이면 유지)   |
| `AccountDelete(id)` | 계정 + 키체인 자격증명 삭제                         |
| `Folders(accID)`    | IMAP 폴더 목록                                      |
| `List(accID, folder)` | 폴더 내 최신 메시지 50건                          |
| `Send(accID, msg)`  | 메일 발송                                           |

## 개발/빌드

```bash
wails3 dev      # 개발 모드 (핫 리로드)
wails3 build    # 프로덕션 빌드 (build/ 산출물)
```

바인딩을 다시 생성할 때:

```bash
wails3 generate bindings -clean=true -ts -i
```

## 새 모듈 추가 방법

1. `internal/modules/<이름>/` 패키지를 만들고 `Service` 구조체를 정의한다.
2. `NewService()` 생성자와 (필요시) `ServiceShutdown()` 훅을 작성한다.
3. `main.go`의 `Services` 슬라이스에 `application.NewService(<service>)`를 추가한다.
4. `wails3 generate bindings`로 프론트엔드 바인딩을 갱신한다.
5. 프론트엔드에서 바인딩을 임포트해 사용한다.

불필요한 모듈은 `main.go`의 등록만 제거하면 바이너리에서 제외된다.