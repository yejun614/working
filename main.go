package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"working/internal/config"
	accountmod "working/internal/modules/account"
	calendarmod "working/internal/modules/calendar"
	clockmod "working/internal/modules/clock"
	documentmod "working/internal/modules/document"
	emailmod "working/internal/modules/email"
	kanbanmod "working/internal/modules/kanban"
	"working/internal/platform"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := config.LoadEnv(); err != nil {
		log.Fatalf("환경변수 초기화 실패: %v", err)
	}

	// 계정 모듈이 먼저 초기화되면서 모듈별로 나뉘어 있던 기존 계정을 한 번 이관한다.
	accountService, err := accountmod.NewService()
	if err != nil {
		log.Fatalf("계정 모듈 초기화 실패: %v", err)
	}

	emailService, err := emailmod.NewService()
	if err != nil {
		log.Fatalf("이메일 모듈 초기화 실패: %v", err)
	}

	calendarService, err := calendarmod.NewService()
	if err != nil {
		log.Fatalf("캘린더 모듈 초기화 실패: %v", err)
	}

	kanbanService, err := kanbanmod.NewService()
	if err != nil {
		log.Fatalf("칸반 모듈 초기화 실패: %v", err)
	}

	documentService, err := documentmod.NewService()
	if err != nil {
		log.Fatalf("문서 모듈 초기화 실패: %v", err)
	}

	clockService, err := clockmod.NewService()
	if err != nil {
		log.Fatalf("시계 모듈 초기화 실패: %v", err)
	}

	app := application.New(application.Options{
		Name:        "working",
		Description: "업무 보조 프로그램 - 모듈식 확장",
		Services: []application.Service{
			application.NewService(accountService),
			application.NewService(emailService),
			application.NewService(calendarService),
			application.NewService(kanbanService),
			application.NewService(documentService),
			application.NewService(clockService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "working",
		Width:  1000,
		Height: 700,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// 한국어 입력기 후보 창이 모니터 왼쪽 위에 뜨는 WebView2 문제를 보정한다.
	platform.FixIMEPosition(window)

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
