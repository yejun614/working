# 작업 지침

## 프로젝트 개요

이 저장소는 Go와 Wails v3를 사용하는 Windows 데스크톱 업무 보조 앱이다.
백엔드는 `internal/` 아래 Go 모듈로 구성하고, 프론트엔드는 `frontend/`의 Vue 3 + TypeScript + Vite로 구성한다.
Wails 바인딩은 `frontend/bindings/`에 생성된다.

## 디렉터리 규칙

- `internal/modules/<name>/`: 기능별 백엔드 모듈
- `internal/config/`, `internal/googleoauth/`, `internal/storage/`: 여러 모듈이 공유하는 기반 기능
- `frontend/src/components/<name>/`: 기능별 Vue 화면과 컴포넌트
- `frontend/bindings/`: Go 서비스에서 생성된 TypeScript 바인딩
- `frontend/public/`: 정적 리소스

## 구현 규칙

- 기존 모듈 경계를 유지하고, 기능 변경은 가능한 한 해당 모듈 안에 둔다.
- 자격증명과 OAuth 토큰 같은 민감한 값은 소스 코드나 일반 설정 파일에 저장하지 않는다. 로컬 개발용 환경변수 예시는 `.env.example`에만 기록하고 실제 `.env`는 커밋하지 않는다.
- Go 코드 변경 후 `gofmt`를 실행한다.
- Wails 서비스의 공개 메서드나 모델을 변경하면 관련 TypeScript 바인딩을 함께 갱신한다.
- 사용자에게 보이는 동작과 복잡한 흐름에는 한국어 주석 또는 doc comment를 추가한다.
- 기존 작업과 무관한 파일은 정리하거나 포맷하지 않는다.

## 검증 명령

백엔드:

```bash
gofmt -w <변경한 Go 파일>
go test ./...
```

프론트엔드:

```bash
cd frontend
npm install
npm run build
```

바인딩을 다시 생성해야 하는 경우:

```bash
wails3 generate bindings -clean=true -ts -i
```

## Git 커밋 규칙

- 커밋 전 `git status`와 staged diff를 확인한다.
- 서로 다른 책임의 변경은 명시적인 경로를 지정해 별도 커밋으로 나눈다.
- `git add .`로 전체 작업 트리를 무분별하게 스테이징하지 않는다.
- 커밋 제목은 Conventional Commit 형식을 사용하고 설명은 한국어로 작성한다.
- 커밋하지 않은 기존 변경이나 도구 산출물을 임의로 삭제하지 않는다.
