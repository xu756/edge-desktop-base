package main

import (
	"changeme/server"
	"embed"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	s := server.New()
	if err := s.Init(assets, appIcon); err != nil {
		log.Fatal(err)
	}
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
}
