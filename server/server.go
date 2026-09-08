// Package server is the public entrypoint for the desktop application foundation.
// Runtime implementation lives in desktop; independent capabilities live in
// settings, localapi and update.
package server

import "changeme/server/desktop"

type Server = desktop.Server

func New() *Server { return desktop.New() }
