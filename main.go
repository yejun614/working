package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"working/internal/config"
	calendarmod "working/internal/modules/calendar"
	emailmod "working/internal/modules/email"
	kanbanmod "working/internal/modules/kanban"
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

	app := application.New(application.Options{
		Name:        "working",
		Description: "업무 보조 프로그램 - 모듈식 확장",
		Services: []application.Service{
			application.NewService(emailService),
			application.NewService(calendarService),
			application.NewService(kanbanService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
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

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
