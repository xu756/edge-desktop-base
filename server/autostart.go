package server

import "github.com/wailsapp/wails/v3/pkg/application"

func applicationAutostartOptions() application.AutostartOptions {
	return application.AutostartOptions{
		Identifier: AppIdentifier,
		Arguments:  []string{"--hidden"},
	}
}
