package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var infoVersionPattern = regexp.MustCompile(`(?m)^  version: "[^"]+"`)

func main() {
	version := strings.TrimSpace(os.Getenv("VERSION"))
	if version == "" {
		version = "0.1.0"
	}
	version = strings.TrimPrefix(version, "v")
	commit := strings.TrimSpace(os.Getenv("COMMIT"))
	if commit == "" {
		commit = "dev"
	}
	buildTime := time.Now().UTC().Format(time.RFC3339)
	if strings.ContainsAny(version+commit, "\"\r\n") {
		panic("invalid build metadata")
	}

	content := fmt.Sprintf(`package server

const (
	AppName           = "Edge Desktop Base"
	AppDescription    = "Reusable Wails v3 desktop application foundation"
	AppIdentifier     = "io.github.xu756.edge-desktop-base"
	UpdateRepository  = "xu756/edgeinfer-node-test"
	DefaultAPIAddress = "127.0.0.1:19876"
)

var (
	Version   = %q
	Commit    = %q
	BuildTime = %q
)
`, version, commit, buildTime)
	if err := os.WriteFile("server/buildinfo.go", []byte(content), 0o644); err != nil {
		panic(err)
	}

	configPath := "build/config.yml"
	config, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	updated := infoVersionPattern.ReplaceAllString(string(config), `  version: "`+version+`"`)
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		panic(err)
	}
}
