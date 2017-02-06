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
	deleteBranchFlags deleteBranchFlagValues
)

type deleteBranchFlagValues struct {
	deleteFlag bool
	branchFlag string
}

var cmdDeleteBranch = &cmdline.Command{
	Runner: jiri.RunnerFunc(runDeleteBranch),
	Name:   "delete-branch",
	Short:  "Deletes branches from jiri projects",
	Long: `
Searches for projects containing specified branch and deletes those branches
if -delete flag is specified, else just prints the project names and branch status
`,
}

func init() {
	flags := &cmdDeleteBranch.Flags
	flags.BoolVar(&deleteBranchFlags.deleteFlag, "delete", false, "Delete specified branch from all the projects.")
	flags.StringVar(&deleteBranchFlags.branchFlag, "branch", "", "Branch to delete.")
}

func runDeleteBranch(jirix *jiri.X, args []string) error {
	if deleteBranchFlags.branchFlag == "" {
		return fmt.Errorf("Please specify branch to delete.")
	}
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
	jirix.TimerPush("Get states")
	states, err := project.GetProjectStates(jirix, localProjects, false)
	if err != nil {
		return err
	}

	jirix.TimerPop()
	projectMap := make(map[project.ProjectKey][]string)
	jirix.TimerPush("Build Map")
	for key, state := range states {
		for _, branch := range state.Branches {
			if branch.Name == deleteBranchFlags.branchFlag {
				git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(state.Project.Path))
				extraCommits, err := git.ExtraCommits(branch.Revision, "origin")
				if err != nil {
					return err
				}
				projectMap[key] = extraCommits
				break
			}
		}
	}
	jirix.TimerPop()

	if len(projectMap) == 0 {
		fmt.Printf("Cannot find any project with branch %q\n", deleteBranchFlags.branchFlag)
		return nil
	}

	jirix.TimerPush("Process")
	warnings := false
	for key, extraCommits := range projectMap {
		localProject := states[key].Project
		relativePath, err := filepath.Rel(cDir, localProject.Path)
		if err != nil {
			return err
		}
		fmt.Printf("Project %v(%v): ", localProject.Name, relativePath)
		git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(localProject.Path))
		if states[key].CurrentBranch.Name == deleteBranchFlags.branchFlag {
			if changes, err := git.HasUncommittedChanges(); err != nil {
				return err
			} else if changes {
				warnings = true
				fmt.Printf(color.Red("Has uncommited changes, will not delete it"))
				fmt.Println()
				continue
			} else {
				remote, ok := remoteProjects[key]
				if !ok {
					fmt.Printf(color.Red("Is on branch to be deleted. Cannot find revision to checkout. Will not delete it"))
					fmt.Println()
					continue
				}
				if deleteBranchFlags.deleteFlag {
					if headRev, err := project.GetHeadRevision(jirix, remote); err != nil {
						return err
					} else {
						if err := git.CheckoutBranch(headRev, gitutil.DetachOpt(true)); err != nil {
							return err
						}
					}
				}
			}
		}
		if deleteBranchFlags.deleteFlag {
			if err := git.DeleteBranch(deleteBranchFlags.branchFlag, gitutil.ForceOpt(true)); err != nil {
				return fmt.Errorf("Error while deleting branch for project %v: %v", localProject.Name, err)
			}
			if len(extraCommits) == 0 {
				fmt.Printf(color.Green("Branch deleted"))
			} else {
				warnings = true
				fmt.Printf(color.Yellow("Branch deleted. It might have left some dangling commits behind"))
			}
		} else {
			if len(extraCommits) == 0 {
				fmt.Printf(color.Green("Clean branch deletion"))
			} else {
				warnings = true
				fmt.Printf(color.Yellow("Branch is not merged to origin. It may leave some dangling commits behind"))
			}
		}
		fmt.Println()
	}
	jirix.TimerPop()

	if warnings {
		fmt.Println(color.Yellow("Please check warnings above"))
	}
	if !deleteBranchFlags.deleteFlag {
		fmt.Println(color.Yellow("No branches were deleted, please run with -delete flag"))
	}
	return nil
}
