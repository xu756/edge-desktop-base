package server

import (
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (s *Server) NewMainWindow() {
	startHidden := hasArgument("--hidden")
	window := s.App.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     AppName,
		Width:     900,
		Height:    650,
		MinWidth:  760,
		MinHeight: 560,
		Hidden:    startHidden,
		URL:       "/",
	})
	s.MainWindow = window

	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if s.quitting.Load() {
			return
		}
		if s.Settings.Get().CloseToTray {
			window.Hide()
			event.Cancel()
			return
		}
		s.quitting.Store(true)
		go s.App.Quit()
	})

	window.Center()
	if !startHidden {
		window.Show()
	}
}

func (s *Server) ShowMainWindow() {
	if s.MainWindow == nil {
		return
	}
	if s.MainWindow.IsMinimised() {
		s.MainWindow.Restore()
	}
	s.MainWindow.Show()
	s.MainWindow.Focus()
}

func hasArgument(target string) bool {
	for _, arg := range os.Args[1:] {
		if strings.EqualFold(strings.TrimSpace(arg), target) {
			return true
		}
	}
	return false
}
