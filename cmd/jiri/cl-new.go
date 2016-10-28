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

// init carries out the package initialization.
func init() {
	cmdNewCL = newCmdNewCL()
}

var cmdNewCL *cmdline.Command

func newCmdNewCL() *cmdline.Command {
	cmdCLUpload = newCmdNewCLUpload("upload", runNewCLUpload)
	return &cmdline.Command{
		Name:     "newcl",
		Short:    "Manage changelists for multiple projects",
		Long:     "Manage changelists for multiple projects.",
		Children: []*cmdline.Command{cmdCLUpload},
	}
}

var (
	cl_ccsFlag          string
	cl_editFlag         bool
	cl_hostFlag         string
	cl_presubmitFlag    string
	cl_remoteBranchFlag string
	cl_reviewersFlag    string
	cl_setTopicFlag     bool
	cl_topicFlag        string
	cl_verifyFlag       bool
)

// Use a factory to avoid an initialization loop between between the
// Runner function and the ParsedFlags field in the Command.
func newCmdNewCLUpload(name string, runner func(*jiri.X, []string) error) *cmdline.Command {
	cmdCLUpload := &cmdline.Command{
		Runner: jiri.RunnerFunc(runner),
		Name:   name,
		Short:  "Upload a changelist for review",
		Long: `Command "upload" uploads all commits of a local branch to Gerrit.
`,
	}
	cmdCLUpload.Flags.StringVar(&cl_ccsFlag, "cc", "", `Comma-seperated list of emails or LDAPs to cc.`)
	cmdCLUpload.Flags.StringVar(&cl_hostFlag, "host", "", `Gerrit host to use.  Defaults to gerrit host specified in manifest.`)
	cmdCLUpload.Flags.StringVar(&cl_presubmitFlag, "presubmit", string(gerrit.PresubmitTestTypeAll),
		fmt.Sprintf("The type of presubmit tests to run. Valid values: %s.", strings.Join(gerrit.PresubmitTestTypes(), ",")))
	cmdCLUpload.Flags.StringVar(&cl_remoteBranchFlag, "remote-branch", "", `Name of the remote branch the CL pertains to, without the leading "origin/".`)
	cmdCLUpload.Flags.StringVar(&cl_reviewersFlag, "r", "", `Comma-seperated list of emails or LDAPs to request review.`)
	cmdCLUpload.Flags.BoolVar(&cl_setTopicFlag, "set-topic", true, `Set Gerrit CL topic.`)
	cmdCLUpload.Flags.StringVar(&cl_topicFlag, "topic", "", `CL topic, defaults to <username>-<branchname>.`)
	cmdCLUpload.Flags.BoolVar(&cl_verifyFlag, "verify", true, `Run pre-push git hooks.`)
	return cmdCLUpload
}

// runNewCLUpload is a wrapper that sets up and runs a review instance across
// multiple projects.
func runNewCLUpload(jirix *jiri.X, _ []string) error {
	git := gitutil.New(jirix.NewSeq())
	remoteBranch := "master"
	if cl_remoteBranchFlag != "" {
		remoteBranch = cl_remoteBranchFlag
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

	host := cl_hostFlag
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
		Ccs:          parseEmails(cl_ccsFlag),
		Edit:         cl_editFlag,
		Host:         hostUrl,
		Presubmit:    gerrit.PresubmitTestType(cl_presubmitFlag),
		RemoteBranch: remoteBranch,
		Remote:       gerritRemote.String(),
		Reviewers:    parseEmails(cl_reviewersFlag),
		Verify:       cl_verifyFlag,
	}
	branch, err := gitutil.New(jirix.NewSeq()).CurrentBranchName()
	if err != nil {
		return err
	}
	opts.Branch = branch
	if cl_setTopicFlag && opts.Topic == "" {
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
