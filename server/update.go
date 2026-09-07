package server

import (
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func (s *Server) GetVersion() string {
	return s.Version
}

func (s *Server) StartUpdateCfg() error {

	gh, err := github.New(github.Config{
		Repository:    "xu756/edgeinfer-node-test", // ← change this
		ChecksumAsset: "SHA256SUMS",                // sibling file with sha256 digests
	})
	if err != nil {
		log.Fatalf("github.New: %v", err)
		return err
	}
	return s.App.Updater.Init(updater.Config{
		CurrentVersion: s.Version,
		Providers:      []updater.Provider{gh},
		CheckInterval:  6 * time.Hour,
	})

}
