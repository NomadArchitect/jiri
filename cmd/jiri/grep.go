// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/project"
)

var cmdGrep = &cmdline.Command{
	Runner: jiri.RunnerFunc(runGrep),
	Name:   "grep",
	Short:  "Search across projects.",
	Long: `
Run git grep across all projects.
`,
	ArgsName: "<query>",
}

func runGrep(jirix *jiri.X, args []string) error {
	all_projects, err := project.LocalProjects(jirix, project.FastScan)
	if err != nil {
		return err
	}

	// TODO(ianloic): run in parallel rather than serially.
	// TODO(ianloic): only run grep on projects under the cwd.
	for _, project := range all_projects {
		relpath, err := filepath.Rel(jirix.Root, project.Path)
		if err != nil {
			return err
		}
		git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(project.Path))
		// TODO(ianloic): allow args to be passed to `git grep`.
		lines, err := git.Grep(args[0])
		if err != nil {
			continue
		}
		for _, line := range lines {
			// TODO(ianloic): higlight the project path part like `repo grep`.
			fmt.Printf("%s/%s\n", relpath, line)
		}
	}

	// TODO(ianloic): fail if all of the sub-greps fail
	return nil
}
