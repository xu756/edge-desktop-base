package server

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *Server) SetMenu() {
	menu := s.App.Menu.New()
	appMenu := menu.AddSubmenu("App")
	appMenu.Add("更新").OnClick(func(*application.Context) {
		go func() {
			if err := s.App.Updater.CheckAndInstall(context.Background()); err != nil {
				s.App.Logger.Error("update", "error", err)
			}
		}()
	})
}
