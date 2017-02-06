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
	dryRun bool
}

var cmdDeleteBranch = &cmdline.Command{
	Runner: jiri.RunnerFunc(runDeleteBranch),
	Name:   "delete-branch",
	Short:  "Deletes branches from jiri projects",
	Long: `
Searches for projects containing specified branch and deletes those branches.
`,
	ArgsName: "<branch>",
	ArgsLong: "<branch> is the branch to delete",
}

func init() {
	flags := &cmdDeleteBranch.Flags
	flags.BoolVar(&deleteBranchFlags.dryRun, "dry-run", false, "Dry run and see what all would be deleted")
}

func runDeleteBranch(jirix *jiri.X, args []string) error {
	if len(args) == 0 {
		return jirix.UsageErrorf("Please specify branch to delete")
	}
	if len(args) > 1 {
		return jirix.UsageErrorf("Please provide only one branch to delete")
	}
	branchToDelete := args[0]
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
	type branchInfo struct {
		extraCommits []string
		branch       project.BranchState
	}
	projectMap := make(map[project.ProjectKey]branchInfo)
	jirix.TimerPush("Build Map")
	for key, state := range states {
		for _, branch := range state.Branches {
			if branch.Name == branchToDelete {
				git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(state.Project.Path))
				extraCommits, err := git.ExtraCommits(branch.Revision, "origin")
				if err != nil {
					return err
				}
				projectMap[key] = branchInfo{extraCommits, branch}
				break
			}
		}
	}
	jirix.TimerPop()

	if len(projectMap) == 0 {
		fmt.Printf("Cannot find any project with branch %q\n", branchToDelete)
		return nil
	}

	jirix.TimerPush("Process")
	warnings := false
	for key, bInfo := range projectMap {
		localProject := states[key].Project
		relativePath, err := filepath.Rel(cDir, localProject.Path)
		if err != nil {
			return err
		}
		fmt.Printf("Project %v(%v): ", localProject.Name, relativePath)
		git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(localProject.Path))
		if states[key].CurrentBranch.Name == branchToDelete {
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
				if !deleteBranchFlags.dryRun {
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
		if !deleteBranchFlags.dryRun {
			if err := git.DeleteBranch(branchToDelete, gitutil.ForceOpt(true)); err != nil {
				return fmt.Errorf("Error while deleting branch for project %v: %v", localProject.Name, err)
			}
			if len(bInfo.extraCommits) == 0 {
				fmt.Printf(color.Green("Branch deleted"))
			} else {
				warnings = true
				fmt.Printf(color.Yellow("Branch deleted. It might have left some dangling commits behind"))
				fmt.Printf(color.Yellow("\nTo restore it run git -C %q branch %v %v", relativePath, bInfo.branch.Name, bInfo.branch.Revision))
			}
		} else {
			if len(bInfo.extraCommits) == 0 {
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
	return nil
}
