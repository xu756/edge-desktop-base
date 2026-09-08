package server

import "github.com/wailsapp/wails/v3/pkg/application"

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
