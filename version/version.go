package version

import (
	"bytes"
	"fmt"
)

var (
	Version string
	BuildTime string
)

func FormattedVersion() string {
	var versionString bytes.Buffer
	if Version != "" {
		fmt.Fprintf(&versionString, "%s", Version)
	}
	if BuildTime != "" {
		fmt.Fprintf(&versionString, " %s", BuildTime)
	}
	return versionString.String()
}
