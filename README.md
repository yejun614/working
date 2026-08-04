<div align="center">

# 🗂️ Working

### 모듈형 Windows 데스크톱 업무 보조 앱

이메일 · 캘린더 · 문서 · 칸반 보드를 하나의 가벼운 앱에서.

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v3-FF4800?logo=wails&logoColor=white)
![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vue.js&logoColor=white)
![TypeScript](https://img.shields.io/badge/TypeScript-5-3178c6?logo=typescript&logoColor=white)
![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)

</div>

---

## 📖 소개

**Working**은 이메일, 캘린더, 문서, 칸반 보드를 한 곳에서 관리할 수 있는
Windows 데스크톱 업무 보조 앱입니다. 각 기능은 독립적인 모듈로 분리되어
있어, 필요한 기능만 켜고 끌 수 있는 모듈식 구조로 되어 있습니다.

- **계정**: 메일과 캘린더가 공유하는 통합 계정 관리. 계정 하나에 메일과
캘린더 기능을 함께 켤 수 있고, Google 계정은 OAuth 토큰 하나로 Gmail과
Google 캘린더를 모두 사용합니다.
- **이메일**: Gmail · Naver · Daum · Outlook · Yahoo · iCloud 등 다양한
제공자 지원. SMTP 발송과 IMAP/Gmail API 수신, 첨부파일 처리, 계정별
캐시를 제공합니다.
- **캘린더**: 로컬 일정 관리와 Google Calendar · Apple iCloud · Outlook
등 CalDAV 서버 연동. OAuth 기반 Google 인증과 일정 캐시 동기화를
지원합니다.
- **문서**: 마크다운 문서 작성과 편집. 본문에 `[[다른 문서 제목]]`을 적으면
옵시디언처럼 문서 사이를 눌러 오갈 수 있고, 어떤 문서가 이 문서를
링크했는지도 함께 보여줍니다.
- **칸반**: 보드 · 컬럼 · 카드 구조의 간단한 칸반 보드. 드래그 앤 드롭으로
카드 순서를 변경하고, 카드 마감일을 캘린더에 읽기 전용 종일 일정으로
표시합니다.

> 🎯 **목표**: 가벼운 네이티브 데스크톱 성능과 오프라인 친화적인 캐시
> 구조로, 브라우저 기반 도구를 대체할 수 있는 개인용 업무 허브를
> 제공합니다.

---

## ✨ 주요 기능

### 👤 계정 모듈


| 기능           | 설명                                            |
| ------------ | --------------------------------------------- |
| 통합 계정        | 계정 하나가 메일 설정과 캘린더 설정을 함께 보유. 필요한 기능만 켜고 끔     |
| 단일 Google 인증 | OAuth 토큰 하나를 Gmail과 Google 캘린더가 공유. 재인증도 한 번  |
| 서버 설정 자동 완성  | 이메일 도메인으로 SMTP/IMAP과 CalDAV URL을 함께 채움         |
| 재인증 안내       | 토큰 만료·철회를 감지해 계정 목록에 경고와 재인증 버튼 표시            |
| 자동 이관        | 기존 모듈별 계정과 키체인 자격증명을 첫 실행 때 통합 계정으로 이관        |


### 📧 이메일 모듈


| 기능            | 설명                                            |
| ------------- | --------------------------------------------- |
| 복수 계정 관리      | 계정 등록/편집/삭제. 자격증명은 OS 키체인에 안전하게 저장            |
| 제공자 자동 입력     | 이메일 도메인으로 SMTP/IMAP 서버 설정 자동 완성               |
| SMTP 발송       | `none` / `STARTTLS` / `TLS` 지원, 첨부파일 MIME 인코딩 |
| IMAP/Gmail 수신 | 폴더 목록 조회, 페이지네이션, multipart 본문 추출             |
| Gmail OAuth   | Google OAuth로 Gmail API를 사용한 수신/발송            |
| 로컬 캐시         | SQLite에 메일 목록/본문 캐시, 오프라인에서도 읽기 가능            |


### 📅 캘린더 모듈


| 기능           | 설명                                                     |
| ------------ | ------------------------------------------------------ |
| 로컬 일정        | CalDAV 서버 없이 로컬 전용 계정으로 일정 관리                          |
| CalDAV 연동    | Google · Apple · Outlook · Yahoo · Nextcloud 표준 CalDAV |
| Google OAuth | PKCE 기반 OAuth 인증, access token 자동 갱신                   |
| iCalendar 표준 | RFC 5545 VEVENT 파싱/직렬화, RRULE 반복 일정 지원                 |
| 일정 캐시        | SyncNow로 서버에서 동기화 후 SQLite 캐시, 화면 전환은 오프라인             |
| 월간 캘린더 뷰     | 월 단위 그리드, 계정별 색상 구분, 날짜 클릭으로 일정 조회                     |


### 📋 칸반 모듈


| 기능       | 설명                                     |
| -------- | -------------------------------------- |
| 보드/컬럼/카드 | 계층 구조의 칸반 보드, 기본 컬럼(할 일/진행 중/완료) 자동 생성 |
| 드래그 앤 드롭 | 컬럼·카드 순서 변경, 백엔드 순서 정규화                |
| 카드 보관/복구 | 삭제 대신 보관(Archive), 언제든 복원 가능           |
| 마감일 연동   | 카드 마감일을 캘린더에 읽기 전용 종일 일정으로 표시          |


### 📝 문서 모듈


| 기능        | 설명                                              |
| --------- | ----------------------------------------------- |
| 마크다운 편집   | TUI Editor 기반 편집기, 본문과 미리보기를 나란히 표시             |
| 위키 링크     | `[[문서 제목]]`을 미리보기에서 눌러 이동. 없는 문서는 다르게 표시하고 눌러 생성 |
| 백링크       | 현재 문서를 링크한 문서 목록을 하단에 표시                        |
| 제목 변경 반영  | 제목을 바꾸면 다른 문서의 링크도 함께 갱신해 끊기지 않음                |
| 자동 저장     | 입력을 멈추면 저장하고 저장 상태를 표시                          |
| 검색        | 제목과 본문으로 문서 목록 필터링                              |


---

## 🛠️ 기술 스택


| 영역    | 기술                                                            |
| ----- | ------------------------------------------------------------- |
| 백엔드   | Go 1.25+, [Wails v3](https://v3.wails.io/)                    |
| 프론트엔드 | Vue 3 + TypeScript + Vite                                     |
| 저장소   | SQLite([modernc.org/sqlite](https://modernc.org/sqlite))      |
| 자격증명  | OS 키체인([go-keyring](https://github.com/zalando/go-keyring))   |
| OAuth | [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2) |
| 캘린더   | [go-ical](https://github.com/emersion/go-ical), CalDAV        |
| 메일    | [go-imap](https://github.com/emersion/go-imap), Gmail API     |
| 문서    | [TUI Editor](https://ui.toast.com/tui-editor)                 |
| 폰트    | [Pretendard](https://github.com/orioncactus/pretendard)       |


---

## 📁 디렉터리 구조

```
working/
├─ main.go                          # 앱 진입점. 사용할 모듈 Service를 등록
├─ internal/
│  ├─ config/                       # 환경변수·데이터 디렉토리 등 공용 설정
│  ├─ googleoauth/                  # Google OAuth 공용 인증 흐름
│  ├─ storage/                      # 공용 SQLite 저장소
│  └─ modules/
│     ├─ account/                   # 통합 계정 모듈 (메일·캘린더 공용)
│     │  ├─ service.go              # Wails Service
│     │  ├─ types/                  # 통합 계정 모델
│     │  └─ store/                  # 계정 목록 + 키체인 + 기존 계정 이관
│     ├─ email/                     # 이메일 모듈
│     │  ├─ service.go              # Wails Service (프론트엔드 API 진입점)
│     │  ├─ gmail/                  # Gmail API 클라이언트
│     │  ├─ imap/                   # IMAP 수신
│     │  ├─ smtp/                   # SMTP 발송
│     │  └─ store/                  # 메일 캐시
│     ├─ calendar/                  # 캘린더 모듈
│     │  ├─ service.go              # Wails Service
│     │  ├─ caldav/                 # CalDAV 클라이언트
│     │  ├─ ical/                   # iCalendar 파싱/직렬화
│     │  └─ store/                  # 일정 캐시
│     ├─ kanban/                    # 칸반 모듈
│     │  ├─ service.go              # Wails Service
│     │  └─ store/                  # 보드/컬럼/카드 저장
│     └─ document/                  # 문서 모듈
│        ├─ service.go              # Wails Service
│        ├─ link.go                 # [[문서 제목]] 링크 파싱/갱신
│        ├─ types/                  # 문서 모델
│        └─ store/                  # 문서 저장
├─ frontend/
│  ├─ src/components/
│  │  ├─ account/                   # 통합 계정 관리 UI
│  │  ├─ email/                     # 이메일 UI
│  │  ├─ calendar/                  # 캘린더 UI
│  │  ├─ kanban/                    # 칸반 UI
│  │  └─ document/                  # 문서 편집 UI
│  ├─ src/theme.ts                  # 공통 테마 (다크/라이트)
│  ├─ public/fonts/                 # Pretendard 폰트
│  └─ bindings/                     # Wails가 자동 생성하는 TS 바인딩
└─ build/                           # 플랫폼별 빌드/패키지 설정
```

---

## 🚀 시작하기

### 사전 요구사항

- [Go 1.25+](https://go.dev/dl/)
- [Node.js](https://nodejs.org/) (프론트엔드 빌드용)
- [Wails v3 CLI](https://v3.wails.io/docs/getting-started/installation/)

### 설치

```bash
git clone https://github.com/<your-username>/working.git
cd working
```

Google OAuth를 사용하려면 루트 디렉터리에 `.env` 파일을 만듭니다.
`.env.example`을 참고해 Google Cloud Console에서 발급받은 OAuth 클라이언트
ID와 보조 비밀값을 입력합니다. `.env`는 `.gitignore`에 의해 커밋되지
않습니다.

```bash
cp .env.example .env
# .env 편집 후
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
```

### 개발 실행

```bash
wails3 dev      # 개발 모드 (핫 리로드)
wails3 build    # 프로덕션 빌드 (build/ 산출물)
```

### 바인딩 재생성

Wails 서비스의 공개 메서드나 모델을 변경한 뒤에는 프론트엔드 바인딩을
갱신합니다.

```bash
wails3 generate bindings -clean=true -ts -i
```

---

## 🧩 새 모듈 추가하기

1. `internal/modules/<이름>/` 패키지를 만들고 `Service` 구조체를 정의합니다.
2. `NewService()` 생성자와 (필요시) `ServiceShutdown()` 훅을 작성합니다.
3. `main.go`의 `Services` 슬라이스에 `application.NewService(<service>)`를
 추가합니다.
4. `wails3 generate bindings`로 프론트엔드 바인딩을 갱신합니다.
5. 프론트엔드에서 바인딩을 임포트해 사용합니다.

불필요한 모듈은 `main.go`의 등록만 제거하면 바이너리에서 제외됩니다.

---

## 🔐 보안

- 비밀번호·OAuth 토큰 등 자격증명은 OS 키체인(Windows Credential Manager)에
저장되며 소스 코드나 평문 파일에 기록되지 않습니다. 통합 계정 모듈이
`working.account` 항목 하나로 관리합니다.
- 계정 메타데이터와 캐시는 사용자 데이터 디렉터리의 SQLite 데이터베이스에
저장됩니다.
- `.env` 파일은 `.gitignore`에 의해 커밋에서 제외됩니다.

---

## 📜 라이선스

이 프로젝트는 [MIT 라이선스](./LICENSE) 하에 배포됩니다.

---

## 🤖 AI 도구 사용 안내

이 저장소의 코드, 문서, 커밋 메시지는 [Claude Code](https://claude.com/claude-code)
등 AI 코딩 도구의 도움을 받아 작성되었습니다. 작업 지침은 `AGENTS.md`를
참고하세요.

<div align="center">

Made with ❤️ in Seoul

</div>