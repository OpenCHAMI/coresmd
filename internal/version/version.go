package version

import "runtime"

// GitCommit stores the latest Git commit hash.
// Set via -ldflags "-X main.GitCommit=$(git rev-parse HEAD)"
var GitCommit string

// BuildTime stores the build timestamp in UTC.
// Set via -ldflags "-X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var BuildTime string

// Version indicates the version of the binary, such as a release number or semantic version.
// Set via -ldflags "-X main.Version=v1.0.0"
var Version string

// GitBranch holds the name of the Git branch from which the build was created.
// Set via -ldflags "-X main.GitBranch=$(git rev-parse --abbrev-ref HEAD)"
var GitBranch string

// GitTag represents the most recent Git tag at build time, if any.
// Set via -ldflags "-X main.GitTag=$(git describe --tags --abbrev=0)"
var GitTag string

// GitState indicates whether the working directory was "clean" or "dirty" (i.e., with uncommitted changes).
// Set via -ldflags "-X main.GitState=$(if git diff-index --quiet HEAD --; then echo 'clean'; else echo 'dirty'; fi)"
var GitState string

// BuildHost stores the hostname of the machine where the binary was built.
// Set via -ldflags "-X main.BuildHost=$(hostname)"
var BuildHost string

// GoVersion captures the Go version used to build the binary.
// Typically, this can be obtained automatically with runtime.Version(), but you can set it manually.
// Set via -ldflags "-X main.GoVersion=$(go version | awk '{print $3}')"
var GoVersion string

// BuildUser is the username of the person or system that initiated the build process.
// Set via -ldflags "-X main.BuildUser=$(whoami)"
var BuildUser string

// Compiler that built the running binary.
var Compiler string = runtime.Compiler

// CPU architecture of the running binary.
var Arch string = runtime.GOARCH

// OS the running binary was built for.
var Os string = runtime.GOOS

// VersionInfo is all of the fields above combined into a map to be logged.
var VersionInfo = map[string]interface{}{
	"version":         Version,
	"tag":             GitTag,
	"commit":          GitCommit,
	"branch":          GitBranch,
	"state":           GitState,
	"build_timestamp": BuildTime,
	"build_host":      BuildHost,
	"build_user":      BuildUser,
	"go_version":      GoVersion,
	"arch":            Arch,
	"os":              Os,
}
