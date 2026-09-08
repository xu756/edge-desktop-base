package server

import "changeme/project"

var appConfig = project.Load()

var (
	AppName           = appConfig.Name
	AppDescription    = appConfig.Description
	AppIdentifier     = appConfig.Identifier
	UpdateRepository  = appConfig.UpdateRepository
	DefaultAPIAddress = appConfig.DefaultAPIAddress
)
