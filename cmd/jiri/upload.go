// Copyright 2015 The Vanadium Authors. All rights reserved.
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
)

var (
	upload_ccsFlag          string
	upload_editFlag         bool
	upload_hostFlag         string
	upload_presubmitFlag    string
	upload_remoteBranchFlag string
	upload_reviewersFlag    string
	upload_setTopicFlag     bool
	upload_topicFlag        string
	upload_verifyFlag       bool
)

var cmdUpload = &cmdline.Command{
	Runner: jiri.RunnerFunc(runUpload),
	Name:   "upload",
	Short:  "Upload a changelist for review",
	Long:   `Command "upload" uploads all commits of a local branch to Gerrit.`,
}

// init carries out the package initialization.
func init() {
	cmdUpload.Flags.StringVar(&upload_ccsFlag, "cc", "", `Comma-seperated list of emails or LDAPs to cc.`)
	cmdUpload.Flags.StringVar(&upload_hostFlag, "host", "", `Gerrit host to use.  Defaults to gerrit host specified in manifest.`)
	cmdUpload.Flags.StringVar(&upload_presubmitFlag, "presubmit", string(gerrit.PresubmitTestTypeAll),
		fmt.Sprintf("The type of presubmit tests to run. Valid values: %s.", strings.Join(gerrit.PresubmitTestTypes(), ",")))
	cmdUpload.Flags.StringVar(&upload_remoteBranchFlag, "remote-branch", "", `Name of the remote branch the CL pertains to, without the leading "origin/".`)
	cmdUpload.Flags.StringVar(&upload_reviewersFlag, "r", "", `Comma-seperated list of emails or LDAPs to request review.`)
	cmdUpload.Flags.BoolVar(&upload_setTopicFlag, "set-topic", true, `Set Gerrit CL topic.`)
	cmdUpload.Flags.StringVar(&upload_topicFlag, "topic", "", `CL topic, defaults to <username>-<branchname>.`)
	cmdUpload.Flags.BoolVar(&upload_verifyFlag, "verify", true, `Run pre-push git hooks.`)
}

// runUpload is a wrapper that sets up and runs a review instance across
// multiple projects.
func runUpload(jirix *jiri.X, _ []string) error {
	git := gitutil.New(jirix.NewSeq())
	remoteBranch := "master"
	if upload_remoteBranchFlag != "" {
		remoteBranch = upload_remoteBranchFlag
	} else {
		if git.IsOnBranch() {
			trackingBranch, err := git.TrackingBranchName()
			if err != nil {
				return err
			}
			if trackingBranch != "" {

				// sometimes if user creates a local branch origin/branch
				// then remote branch is represented as remotes/origin/branch
				originIndex := strings.Index(trackingBranch, "origin/")
				if originIndex != -1 {
					trackingBranch = trackingBranch[originIndex+len("origin/"):]
				}
				remoteBranch = trackingBranch
			}
		}
	}
	p, err := currentProject(jirix)
	if err != nil {
		return err
	}

	host := upload_hostFlag
	if host == "" {
		if p.GerritHost == "" {
			return fmt.Errorf("No gerrit host found.  Please use the '--host' flag, or add a 'gerrithost' attribute for project %q.", p.Name)
		}
		host = p.GerritHost
	}
	hostUrl, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("invalid Gerrit host %q: %v", host, err)
	}
	projectRemoteUrl, err := url.Parse(p.Remote)
	if err != nil {
		return fmt.Errorf("invalid project remote: %v", p.Remote, err)
	}
	gerritRemote := *hostUrl
	gerritRemote.Path = projectRemoteUrl.Path
	opts := gerrit.CLOpts{
		Ccs:          parseEmails(upload_ccsFlag),
		Edit:         upload_editFlag,
		Host:         hostUrl,
		Presubmit:    gerrit.PresubmitTestType(upload_presubmitFlag),
		RemoteBranch: remoteBranch,
		Remote:       gerritRemote.String(),
		Reviewers:    parseEmails(upload_reviewersFlag),
		Verify:       upload_verifyFlag,
	}
	branch, err := gitutil.New(jirix.NewSeq()).CurrentBranchName()
	if err != nil {
		return err
	}
	opts.Branch = branch
	if upload_setTopicFlag && opts.Topic == "" {
		opts.Topic = fmt.Sprintf("%s-%s", os.Getenv("USER"), branch) // use <username>-<branchname> as the default
	}
	if opts.Presubmit == gerrit.PresubmitTestType("") {
		opts.Presubmit = gerrit.PresubmitTestTypeAll // use gerrit.PresubmitTestTypeAll as the default
	}
	if err := gerrit.Push(jirix.NewSeq(), opts); err != nil {
		return gerritError(err.Error())
	}
	return nil
}
