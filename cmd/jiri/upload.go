// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/gerrit"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/project"
)

var (
	uploadCcsFlag          string
	uploadEditFlag         bool
	uploadHostFlag         string
	uploadPresubmitFlag    string
	uploadRemoteBranchFlag string
	uploadReviewersFlag    string
	uploadTopicFlag        string
	uploadVerifyFlag       bool
	uploadRebase           bool
	uploadMultipartFlag    bool
)

var cmdUpload = &cmdline.Command{
	Runner: jiri.RunnerFunc(runUpload),
	Name:   "upload",
	Short:  "Upload a changelist for review",
	Long:   `Command "upload" uploads all commits of a local branch to Gerrit.`,
}

func init() {
	cmdUpload.Flags.StringVar(&uploadCcsFlag, "cc", "", `Comma-separated list of emails or LDAPs to cc.`)
	cmdUpload.Flags.StringVar(&uploadHostFlag, "host", "", `Gerrit host to use.  Defaults to gerrit host specified in manifest.`)
	cmdUpload.Flags.StringVar(&uploadPresubmitFlag, "presubmit", string(gerrit.PresubmitTestTypeAll),
		fmt.Sprintf("The type of presubmit tests to run. Valid values: %s.", strings.Join(gerrit.PresubmitTestTypes(), ",")))
	cmdUpload.Flags.StringVar(&uploadReviewersFlag, "r", "", `Comma-separated list of emails or LDAPs to request review.`)
	cmdUpload.Flags.StringVar(&uploadTopicFlag, "topic", "", `CL topic. Default for multipart CL would be <username>-<branchname>`)
	cmdUpload.Flags.BoolVar(&uploadVerifyFlag, "verify", true, `Run pre-push git hooks.`)
	cmdUpload.Flags.BoolVar(&uploadRebase, "rebase", false, `Run rebase before pushing.`)
	cmdUpload.Flags.BoolVar(&uploadMultipartFlag, "multipart", false, `Send multipart CL.`)
}

// runUpload is a wrapper that pushes the changes to gerrit for review.
func runUpload(jirix *jiri.X, _ []string) error {
	p, err := currentProject(jirix)
	if err != nil {
		return err
	}
	git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(p.Path))
	if !git.IsOnBranch() {
		return fmt.Errorf("Current project is not on any branch.")
	}

	currentBranch, err := git.CurrentBranchName()
	if err != nil {
		return err
	}

	var projectsToProcess []project.Project
	topic := uploadTopicFlag
	if uploadMultipartFlag {
		projects, err := project.LocalProjects(jirix, project.FastScan)
		if err != nil {
			return err
		}
		for _, project := range projects {
			git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(project.Path))
			if git.IsOnBranch() {
				branch, err := git.CurrentBranchName()
				if err != nil {
					return err
				}
				if currentBranch == branch {
					projectsToProcess = append(projectsToProcess, project)
				}
			}
		}
		if topic == "" {
			topic = fmt.Sprintf("%s-%s", os.Getenv("USER"), currentBranch) // use <username>-<branchname> as the default
		}

	} else {
		if project, err := currentProject(jirix); err != nil {
			return err
		} else {
			projectsToProcess = append(projectsToProcess, project)
		}
	}
	type GerritPushOption struct {
		project.Project
		gerrit.CLOpts
	}
	var gerritPushOptions []GerritPushOption
	for _, project := range projectsToProcess {
		git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(project.Path))
		if uploadRebase {
			if changes, err := git.HasUncommittedChanges(); err != nil {
				return err
			} else if changes {
				return fmt.Errorf("project has uncommited changes, please commit them or stash them. Cannot rebase before pushing.")
			}
		}
		remoteBranch, err := git.RemoteBranchName()
		if err != nil {
			return err
		}
		if remoteBranch == "" {
			return fmt.Errorf("For project %q, current branch is un-tracked or tracks a local un-tracked branch.", project.Name)
		}

		host := uploadHostFlag
		if host == "" {
			if project.GerritHost == "" {
				return fmt.Errorf("No gerrit host found.  Please use the '--host' flag, or add a 'gerrithost' attribute for project %q.", project.Name)
			}
			host = project.GerritHost
		}
		hostUrl, err := url.Parse(host)
		if err != nil {
			return fmt.Errorf("invalid Gerrit host for project(%v) %q: %v", project.Name, host, err)
		}
		projectRemoteUrl, err := url.Parse(project.Remote)
		if err != nil {
			return fmt.Errorf("invalid project remote for project(%v): %v", project.Name, project.Remote, err)
		}
		gerritRemote := *hostUrl
		gerritRemote.Path = projectRemoteUrl.Path
		opts := gerrit.CLOpts{
			Ccs:          parseEmails(uploadCcsFlag),
			Edit:         uploadEditFlag,
			Host:         hostUrl,
			Presubmit:    gerrit.PresubmitTestType(uploadPresubmitFlag),
			RemoteBranch: remoteBranch,
			Remote:       gerritRemote.String(),
			Reviewers:    parseEmails(uploadReviewersFlag),
			Verify:       uploadVerifyFlag,
			Topic:        topic,
			Branch:       currentBranch,
		}

		if opts.Presubmit == gerrit.PresubmitTestType("") {
			opts.Presubmit = gerrit.PresubmitTestTypeAll
		}
		gerritPushOptions = append(gerritPushOptions, GerritPushOption{project, opts})
	}

	// Rebase all projects before pushing
	if uploadRebase {
		for _, gerritPushOption := range gerritPushOptions {
			git := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(gerritPushOption.Project.Path))
			if err := git.Fetch("", gitutil.AllOpt(true), gitutil.PruneOpt(true)); err != nil {
				return err
			}
			trackingBranch, err := git.TrackingBranchName()
			if err != nil {
				return err
			}
			if err = git.Rebase(trackingBranch); err != nil {
				if err := git.RebaseAbort(); err != nil {
					return err
				}
				return fmt.Errorf("For project %q(%v), not able to rebase the branch to %v, please rebase manually", gerritPushOption.Project.Name, gerritPushOption.Project.Path, trackingBranch)
			}
		}
	}

	for _, gerritPushOption := range gerritPushOptions {
		fmt.Printf("Pushing project(%v)\n", gerritPushOption.Project.Name)
		if err := gerrit.Push(jirix.NewSeq().Dir(gerritPushOption.Project.Path), gerritPushOption.CLOpts); err != nil {
			if strings.Contains(err.Error(), "(no new changes)") {
				if gitErr, ok := err.(gitutil.GitError); ok {
					fmt.Printf("%v", gitErr.Output)
					fmt.Printf("%v", gitErr.ErrorOutput)
				} else {
					return gerritError(err.Error())
				}
			} else {
				return gerritError(err.Error())
			}
		}
		fmt.Println()
	}
	return nil
}
