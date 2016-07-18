// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/fuchsia.googlesource.com/jiri/cmdline/testdata/gendoc.go -env=CMDLINE_PREFIX=jiri .

package main

import (
	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/profiles/profilescmdline"
	"fuchsia.googlesource.com/jiri/profiles/profilesmanager"
	"fuchsia.googlesource.com/jiri/tool"
	"fuchsia.googlesource.com/jiri/cmdline"

	// Add profile manager implementations here.
	"fuchsia.googlesource.com/jiri/profiles/profilescmdline/internal/example"
)

// commandLineDriver implements the command line for the 'profile-v23'
// subcommand.
var commandLineDriver = &cmdline.Command{
	Name:  "profile-i1",
	Short: "Manage i1 profiles",
	Long:  profilescmdline.HelpMsg(),
}

func main() {
	profilesmanager.Register(example.New("i1", "eg"))
	profilescmdline.RegisterManagementCommands(commandLineDriver, true, "i1", jiri.ProfilesDBDir, jiri.ProfilesRootDir)
	tool.InitializeRunFlags(&commandLineDriver.Flags)
	cmdline.Main(commandLineDriver)
}
