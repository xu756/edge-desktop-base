package server

import "github.com/wailsapp/wails/v3/pkg/application"

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
