// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package project

import (
	"fmt"
	"path/filepath"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/runutil"
	"fuchsia.googlesource.com/jiri/tool"
)

type BranchState struct {
	HasGerritMessage  bool
	Name              string
	Revision          string
	TrackingBranch    string
	TrackingBranchRev string
}

type ProjectState struct {
	Branches       []BranchState
	CurrentBranch  BranchState
	HasUncommitted bool
	HasUntracked   bool
	Project        Project
}

func setProjectState(jirix *jiri.X, state *ProjectState, checkDirty bool, ch chan<- error) {
	var err error
	scm := gitutil.New(jirix.NewSeq(), gitutil.RootDirOpt(state.Project.Path))
	branches, currentBranch, err := scm.GetBranches()
	if err != nil {
		ch <- err
		return
	}
	m, err := scm.GetAllBranchesInfo()
	if err != nil {
		ch <- err
		return
	}
	state.CurrentBranch = BranchState{Name: ""}
	for _, branch := range branches {
		file := filepath.Join(state.Project.Path, jiri.ProjectMetaDir, branch, ".gerrit_commit_message")
		hasFile := true
		if _, err := jirix.NewSeq().Stat(file); err != nil {
			if !runutil.IsNotExist(err) {
				ch <- err
				return
			}
			hasFile = false
		}
		b := BranchState{
			Name:             branch,
			HasGerritMessage: hasFile,
		}
		if v, ok := m[gitutil.LocalType+"/"+branch]; ok {
			b.Revision = string(v.Revision)
			b.TrackingBranch = string(v.TrackingBranch)
			if b.TrackingBranch != "" {
				if v, ok := m[gitutil.RemoteType+"/"+b.TrackingBranch]; ok {
					b.TrackingBranchRev = string(v.Revision)
				}
			}
		}
		state.Branches = append(state.Branches, b)
		if currentBranch == branch {
			state.CurrentBranch = b
		}
	}
	if checkDirty {
		state.HasUncommitted, err = scm.HasUncommittedChanges()
		if err != nil {
			ch <- err
			return
		}
		state.HasUntracked, err = scm.HasUntrackedFiles()
		if err != nil {
			ch <- err
			return
		}
	}
	ch <- nil
}

func GetProjectStates(jirix *jiri.X, projects Projects, checkDirty bool) (map[ProjectKey]*ProjectState, error) {
	states := make(map[ProjectKey]*ProjectState, len(projects))
	sem := make(chan error, len(projects))
	for key, project := range projects {
		state := &ProjectState{
			Project: project,
		}
		states[key] = state
		// jirix is not threadsafe, so we make a clone for each goroutine.
		go setProjectState(jirix.Clone(tool.ContextOpts{}), state, checkDirty, sem)
	}
	for _ = range projects {
		err := <-sem
		if err != nil {
			return nil, err
		}
	}
	return states, nil
}

func GetProjectState(jirix *jiri.X, key ProjectKey, checkDirty bool) (*ProjectState, error) {
	projects, err := LocalProjects(jirix, FastScan)
	if err != nil {
		return nil, err
	}
	sem := make(chan error, 1)
	for k, project := range projects {
		if k == key {
			state := &ProjectState{
				Project: project,
			}
			setProjectState(jirix, state, checkDirty, sem)
			return state, <-sem
		}
	}
	return nil, fmt.Errorf("failed to find project key %v", key)
}
