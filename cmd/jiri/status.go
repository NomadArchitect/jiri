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
	"fuchsia.googlesource.com/jiri/color"
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
	commits bool
}

var cmdStatus = &cmdline.Command{
	Runner: jiri.RunnerFunc(runStatus),
	Name:   "status",
	Short:  "Prints status of all the projects",
	Long: `
Prints status for the the projects. It runs git status -s across all the projects
and prints it if there are some changes. It also shows status if the project is on
a rev other then the one according to manifest.
`,
}

func init() {
	flags := &cmdStatus.Flags
	flags.BoolVar(&statusFlags.changes, "changes", true, "Display projects with tracked or un-tracked changes.")
	flags.BoolVar(&statusFlags.notHead, "not-head", true, "Display projects that are not on HEAD/pinned revisions.")
	flags.BoolVar(&statusFlags.commits, "commits", true, "Display commits not merged with remote. This only works with branch flag.")
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
		if statusFlags.branch == "" || (statusFlags.branch == state.CurrentBranch.Name) {
			if changes, headRev, extraCommits, err := getStatus(jirix, localProject, remoteProject); err != nil {
				return fmt.Errorf("Error while getting status for project %q :%v", localProject.Name, err)
			} else {
				revisionMessage := ""
				if statusFlags.notHead {
					if headRev == "" {
						revisionMessage = "Can't find project in manifest, can't get revision status"
					} else if headRev != state.CurrentBranch.Revision {
						revisionMessage = fmt.Sprintf("Should be on revision %q, but is on revision %q", headRev, state.CurrentBranch.Revision)
					}
				}
				if statusFlags.branch != "" ||
					(statusFlags.branch == "" && (changes != "" || revisionMessage != "")) {
					relativePath, err := filepath.Rel(cDir, localProject.Path)
					if err != nil {
						return err
					}
					fmt.Printf(color.Green("%v(%v): %v", localProject.Name, relativePath, revisionMessage))
					fmt.Println()
					branch := state.CurrentBranch.Name
					if branch == "" {
						branch = fmt.Sprintf("DETACHED-HEAD(%v)", state.CurrentBranch.Revision)
					}
					fmt.Printf(color.Yellow("Branch: %v\n", branch))
					if statusFlags.branch != "" && len(extraCommits) != 0 {
						fmt.Printf(color.Magenta("Commits: %v commit(s) not merged to remote\n", len(extraCommits)))
						for _, commitLog := range extraCommits {
							fmt.Println(color.Magenta(commitLog))
						}
					}
					if changes != "" {
						fmt.Println(color.Cyan(changes))
					}
					fmt.Println()
				}
			}
		}
	}
	return nil
}

func getStatus(jirix *jiri.X, local project.Project, remote project.Project) (string, string, []string, error) {
	revision := ""
	git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(local.Path))
	changes := ""
	var extraCommits []string
	if statusFlags.changes {
		var err error
		changes, err = git.ShortStatus()
		if err != nil {
			return "", "", nil, err
		}
	}
	if statusFlags.notHead && remote.Name != "" {
		if headRev, err := project.GetHeadRevision(jirix, remote); err != nil {
			return "", "", nil, err
		} else {
			if headRev, err = git.CurrentRevisionOfBranch(headRev); err != nil {
				return "", "", nil, err
			}
			revision = headRev
		}
	}

	if statusFlags.branch != "" && statusFlags.commits {
		commits, err := git.ExtraCommits(statusFlags.branch, "origin")
		if err != nil {
			return "", "", nil, err
		}
		for _, commit := range commits {
			if log, err := git.OneLineLog(commit); err != nil {
				return "", "", nil, err
			} else {
				extraCommits = append(extraCommits, log)
			}
		}

	}
	return changes, revision, extraCommits, nil
}
