package app

var (
	Version    = "v0.0.0-dev" //nolint:gochecknoglobals // special global for build version
	CommitHash = ""           //nolint:gochecknoglobals // special global for build commit
	BuildTime  string         //nolint:gochecknoglobals // special global for build time
	Platform   = "local"      //nolint:gochecknoglobals // special global for build platform
)

type Info struct {
	BuildTime  string
	CommitHash string
	Platform   string
	Version    string
}

func (a *App) GetBuildInfoItem() (value string) {
	return a.buildInfoItems()[a.activeBuildInfoItem]
}

func (a *App) GetPreviousBuildInfoItem() (value string) {
	a.activeBuildInfoItem++
	if a.activeBuildInfoItem >= len(a.buildInfoItems()) {
		a.activeBuildInfoItem = 0
	}

	return a.buildInfoItems()[a.activeBuildInfoItem]
}

func (a *App) GetNextBuildInfoItem() (value string) {
	a.activeBuildInfoItem--
	if a.activeBuildInfoItem < 0 {
		a.activeBuildInfoItem = len(a.buildInfoItems()) - 1
	}

	return a.buildInfoItems()[a.activeBuildInfoItem]
}

func (a *App) buildInfoItems() []string {
	return []string{
		"Version\n" + Version,
		"Commit Hash\n" + CommitHash,
		"Build Time\n" + BuildTime,
		"Platform\n" + Platform,
		"IP Address\n" + a.ipAddress,
	}
}
