// Package exitcode defines exit codes used by the application.
package exitcode

type Code int

const (
	Success          Code = 0
	GeneralErr       Code = 1
	RestartApp       Code = 29
	RestartGTClient  Code = 30
	SetupRequired    Code = 31
	SetupMode        Code = 32
	CommandUsageErr  Code = 64
	DataFormatErr    Code = 65
	InternalErr      Code = 70
	SystemErr        Code = 71
	PermissionDenied Code = 77
	ConfigErr        Code = 78
)
