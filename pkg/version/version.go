package version

import "strings"

func FullVersion() string {
	sb := strings.Builder{}
	sb.Grow(len(Version) + len(Distribution) + len(Commit) + len("-") + len("+"))

	sb.WriteString(Version)
	sb.WriteString("+")
	sb.WriteString(Commit)
	sb.WriteString("-")
	sb.WriteString(Distribution)

	return sb.String()

}

var Version = "v0.0.0-dev"
var Commit = ""
var Distribution = "oss"
