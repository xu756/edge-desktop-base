package main

import (
	"changeme/server"
	"embed"
	"fmt"
)

const currentVersion = "0.0.1"

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	server := server.New(currentVersion)
	if err := server.Init(assets); err != nil {

		server.App.Logger.Error(fmt.Sprintf("Server.Init: %v", err))
	}

	if err := server.Start(); err != nil {
		server.App.Logger.Error(fmt.Sprintf("Server.Start: %v", err))
	}

}
