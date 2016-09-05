// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
