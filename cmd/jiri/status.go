// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/project"
)

var (
	statusFlags statusFlagValues
)

type statusFlagValues struct {
	changes bool
	notHead bool
	branch  string
}

var cmdStatus = &cmdline.Command{
	Runner: jiri.RunnerFunc(runStatus),
	Name:   "status",
	Short:  "Prints status of all the projects",
	Long: `
Prints status for the the projects. It runs git status -s across all the projects
and prints it if there are some changes. It also shows status if the project is on
a rev other then the one according to manifest(Named as JIRI_HEAD in git)
`,
}

func init() {
	flags := &cmdStatus.Flags
	flags.BoolVar(&statusFlags.changes, "changes", true, "Display projects with tracked or un-tracked changes.")
	flags.BoolVar(&statusFlags.notHead, "not-head", true, "Display projects that are not on HEAD/pinned revisions.")
	flags.StringVar(&statusFlags.branch, "branch", "", "Display all projects only on this branch along with thier status.")
}

func runStatus(jirix *jiri.X, args []string) error {
	localProjects, err := project.LocalProjects(jirix, project.FastScan)
	if err != nil {
		return err
	}
	remoteProjects, _, _, err := project.LoadUpdatedManifest(jirix, localProjects, true)
	if err != nil {
		return err
	}
	cDir, err := os.Getwd()
	if err != nil {
		return err
	}
	states, err := project.GetProjectStates(jirix, localProjects, false)
	if err != nil {
		return err
	}
	for key, localProject := range localProjects {
		remoteProject, _ := remoteProjects[key]
		state, ok := states[key]
		if !ok {
			// this should not happen
			panic(fmt.Sprintf("State not found for project %q", localProject.Name))
		}
		changes, headRev, err := getStatus(jirix, localProject, remoteProject)
		if err != nil {
			return err
		}
		revisionMessage := ""
		if statusFlags.notHead {
			if headRev == "" {
				revisionMessage = "Can't find project in manifest, can't get revision status"
			} else if headRev != state.CurrentBranch.Revision {
				revisionMessage = fmt.Sprintf("Should be on revision %q, but is on revision %q", headRev, state.CurrentBranch.Revision)
			}
		}
		if (statusFlags.branch != "" && statusFlags.branch == state.CurrentBranch.Name) ||
			(statusFlags.branch == "" && (changes != "" || revisionMessage != "")) {
			relativePath, err := filepath.Rel(cDir, localProject.Path)
			if err != nil {
				return err
			}
			fmt.Printf("%v(%v): %v", localProject.Name, relativePath, revisionMessage)
			fmt.Println()
			branch := state.CurrentBranch.Name
			if branch == "" {
				branch = fmt.Sprintf("DETACHED-HEAD(%v)", state.CurrentBranch.Revision)
			}
			fmt.Printf("Branch: %v\n", branch)
			if changes != "" {
				fmt.Println(changes)
			}
			fmt.Println()
		}

	}
	return nil
}

func getStatus(jirix *jiri.X, local project.Project, remote project.Project) (string, string, error) {
	revision := ""
	git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(local.Path))
	changes := ""
	if statusFlags.changes {
		var err error
		changes, err = git.ShortStatus()
		if err != nil {
			return "", "", err
		}
	}
	if statusFlags.notHead && remote.Name != "" {
		if headRev, err := project.GetHeadRevision(jirix, remote); err != nil {
			return "", "", err
		} else {
			if expectedRev, err = g.CurrentRevisionForRef(expectedRev); err != nil {
				return "", "", fmt.Errorf("Cannot find revision for ref %q for project %q: %v", expectedRev, local.Name, err)
			}
			revision = headRev
		}
	}
	return changes, revision, nil
}
