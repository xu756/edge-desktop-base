package server

import "changeme/project"

var appConfig = project.Load()

var (
	AppName             = appConfig.Name
	AppDescription      = appConfig.Description
	AppIdentifier       = appConfig.Identifier
	UpdateRepositoryURL = appConfig.UpdateRepositoryURL
	UpdateBranch        = appConfig.UpdateBranch
	DefaultAPIAddress   = appConfig.DefaultAPIAddress
)
