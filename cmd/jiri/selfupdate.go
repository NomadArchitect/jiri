// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
)

// cmdSelfUpdate represents the "jiri update" command.
var cmdSelfUpdate = &cmdline.Command{
	Runner: jiri.RunnerFunc(runSelfUpdate),
	Name:   "selfupdate",
	Short:  "Update jiri tool",
	Long: `
Updates jiri tool and replaces current one with the latest`,
}

func runSelfUpdate(jirix *jiri.X, args []string) error {
	if len(args) > 0 {
		return jirix.UsageErrorf("unexpected number of arguments")
	}

	if err := jiri.Update(true, true); err != nil {
		return fmt.Errorf("Update failed: %v", err)
	}
	fmt.Println("Tool updated.")
	return nil
}
