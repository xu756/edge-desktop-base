package desktop

import (
	"changeme/server/settings"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func (s *Server) NewMainWindow() {
	startHidden := shouldStartHidden(os.Args[1:], s.Settings.Get())
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

func shouldStartHidden(args []string, cfg settings.Settings) bool {
	hidden, autostart := false, false
	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--hidden":
			hidden = true
		case "--autostart":
			autostart = true
		}
	}
	if autostart {
		return !cfg.AutoStartShowWindow
	}
	return hidden
}

func (s *Server) SetTray(icon []byte) {
	tray := s.App.SystemTray.New()
	if len(icon) > 0 {
		tray.SetIcon(icon)
	}
	tray.SetLabel(AppName)
	tray.SetTooltip(AppName)

	menu := s.App.Menu.New()
	menu.Add("打开主窗口").OnClick(func(*application.Context) {
		s.ShowMainWindow()
	})
	menu.Add("检查更新").OnClick(func(*application.Context) {
		if err := s.CheckForUpdates(); err != nil {
			s.App.Logger.Error("check update", "error", err)
		}
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(*application.Context) {
		s.Quit()
	})

	tray.SetMenu(menu)
	tray.OnClick(func() {
		s.ShowMainWindow()
	})
	s.Tray = tray
}

func applicationAutostartOptions(showWindow bool) application.AutostartOptions {
	args := []string{"--autostart"}
	if !showWindow {
		args = append(args, "--hidden")
	}
	return application.AutostartOptions{
		Identifier: AppIdentifier,
		Arguments:  args,
	}
}
