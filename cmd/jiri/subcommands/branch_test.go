// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subcommands

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.fuchsia.dev/jiri/gitutil"
	"go.fuchsia.dev/jiri/jiritest"
	"go.fuchsia.dev/jiri/project"
)

func createBranchCommits(t *testing.T, fake *jiritest.FakeJiriRoot, localProjects []project.Project) {
	for i, localProject := range localProjects {
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file1"+strconv.Itoa(i), "file1"+strconv.Itoa(i))
	}
}

func createBranchProjects(t *testing.T, fake *jiritest.FakeJiriRoot, numProjects int) []project.Project {
	localProjects := []project.Project{}
	for i := 0; i < numProjects; i++ {
		name := fmt.Sprintf("project-%d", i)
		path := fmt.Sprintf("path-%d", i)
		if err := fake.CreateRemoteProject(name); err != nil {
			t.Fatal(err)
		}
		p := project.Project{
			Name:   name,
			Path:   filepath.Join(fake.X.Root, path),
			Remote: fake.Projects[name],
		}
		localProjects = append(localProjects, p)
		if err := fake.AddProject(p); err != nil {
			t.Fatal(err)
		}
	}
	createBranchCommits(t, fake, localProjects)
	return localProjects
}

func TestBranch(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 8
	localProjects := createBranchProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	testBranch := "testBranch"
	testBranch2 := "testBranch2"

	defaultWant := ""
	branchWant := ""
	listWant := ""
	relativePath := make([]string, numProjects)
	for i, p := range localProjects {
		var err error
		relativePath[i], err = filepath.Rel(fake.X.Root, p.Path)
		if err != nil {
			t.Fatal(err)
		}
	}
	// current branch is not testBranch
	i := 0
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch("main", localProjects[i].GitSubmodules, false)
	branchWant = fmt.Sprintf("%s%s(%s)\n", branchWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): %s, *main\n\n", defaultWant, testBranch)

	i = 2
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CreateBranch(testBranch2)
	branchWant = fmt.Sprintf("%s%s(%s)\n", branchWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): %s, %s\n\n", defaultWant, testBranch, testBranch2)

	i = 3
	gitLocals[i].CreateBranch(testBranch)
	branchWant = fmt.Sprintf("%s%s(%s)\n", branchWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): %s\n\n", defaultWant, testBranch)

	// current branch is test branch
	i = 1
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)
	gitLocals[i].CreateBranch(testBranch2)
	listWant = fmt.Sprintf("%s%s(%s)\n", listWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): *%s, %s\n\n", defaultWant, testBranch, testBranch2)

	i = 6
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CreateBranch("main")
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)
	listWant = fmt.Sprintf("%s%s(%s)\n", listWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): *%s, main\n\n", defaultWant, testBranch)

	i = 4
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)
	gitLocals[i].CreateBranch(testBranch2)
	listWant = fmt.Sprintf("%s%s(%s)\n", listWant, localProjects[i].Name, relativePath[i])
	branchWant = fmt.Sprintf("%s%s", branchWant, listWant)
	defaultWant = fmt.Sprintf("%sProject: %s(%s)\n", defaultWant, localProjects[i].Name, relativePath[i])
	defaultWant = fmt.Sprintf("%sBranch(es): *%s, %s\n\n", defaultWant, testBranch, testBranch2)

	// Run default
	if got := executeBranch(t, fake, branchCmd{}); !equalDefaultBranchOut(got, defaultWant) {
		t.Errorf("got %s, want %s", got, defaultWant)
	}
	// Run with branch
	if got := executeBranch(t, fake, branchCmd{}, testBranch); !equalBranchOut(got, branchWant) {
		t.Errorf("got %s, want %s", got, branchWant)
	}

	// Run with -list
	if got := executeBranch(t, fake, branchCmd{list: true}, testBranch); !equalBranchOut(got, listWant) {
		t.Errorf("got %s, want %s", got, listWant)
	}
}

func TestDeleteBranchWithProjectConfig(t *testing.T) {
	t.Parallel()

	testDeleteBranchWithProjectConfig(t, false)
	testDeleteBranchWithProjectConfig(t, true)
}

func testDeleteBranchWithProjectConfig(t *testing.T, overridePC bool) {
	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 4
	localProjects := createBranchProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	testBranch := "testBranch"

	// Test case when new test branch is on HEAD
	i := 0
	gitLocals[i].CreateBranch(testBranch)
	lc := project.LocalConfig{NoUpdate: true}
	project.WriteLocalConfig(fake.X, localProjects[i], lc)

	// Test when git branch -d fails
	i = 1
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	gitLocals[i].CheckoutBranch("main", localProjects[i].GitSubmodules, false)

	// Test when current branch is test branch
	i = 2
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)

	// project-3 has no test branch

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}

	executeBranch(t, fake, branchCmd{
		delete:                true,
		overrideProjectConfig: overridePC,
	}, testBranch)

	states, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		state, _ := states[localProject.Key()]
		branchFound := false
		for _, branch := range state.Branches {
			if branch.Name == testBranch {
				branchFound = true
				break
			}
		}
		if (!overridePC && i == 0) || i == 1 || i == 2 {
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		} else {
			if branchFound {
				t.Errorf("project %q should not contain branch %q", localProject.Name, testBranch)
			}

		}
	}
}

func TestDeleteBranch(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 4
	localProjects := createBranchProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	testBranch := "testBranch"

	// Test case when new test branch is on HEAD
	i := 0
	gitLocals[i].CreateBranch(testBranch)

	// Test when git branch -d fails
	i = 1
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	gitLocals[i].CheckoutBranch("main", localProjects[i].GitSubmodules, false)

	// Test when current branch is test branch
	i = 2
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch, localProjects[i].GitSubmodules, false)

	// project-3 has no test branch

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}

	// Run on default, should not delete any branch
	executeBranch(t, fake, branchCmd{}, testBranch)

	states, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		state, _ := states[localProject.Key()]
		branchFound := false
		for _, branch := range state.Branches {
			if branch.Name == testBranch {
				branchFound = true
				break
			}
		}
		if i == 0 || i == 1 || i == 2 {
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		}
	}

	executeBranch(t, fake, branchCmd{delete: true}, testBranch)

	states, err = project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		state, _ := states[localProject.Key()]
		branchFound := false
		for _, branch := range state.Branches {
			if branch.Name == testBranch {
				branchFound = true
				break
			}
		}
		if i == 1 || i == 2 {
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		} else {
			if branchFound {

				t.Errorf("project %q should not contain branch %q", localProject.Name, testBranch)
			}

		}
	}

	executeBranch(t, fake, branchCmd{forceDelete: true}, testBranch)

	states, err = project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		state, _ := states[localProject.Key()]
		branchFound := false
		for _, branch := range state.Branches {
			if branch.Name == testBranch {
				branchFound = true
				break
			}
		}
		if i == 2 {
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		} else {
			if branchFound {

				t.Errorf("project %q should not contain branch %q", localProject.Name, testBranch)
			}

		}
	}
}

var r = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(strlen int) string {
	const chars = "abcde0123456789ABCDE"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return string(result)
}

func generateChangeIds(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = "I" + randomString(40)
	}
	return ids
}

func TestDeleteMergedClsBranch(t *testing.T) {
	t.Parallel()

	mergedIds := generateChangeIds(2)
	unmergedIds := generateChangeIds(1)
	localIds := generateChangeIds(1)
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/changes/", func(rw http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if id, ok := r.Form["q"]; ok {
			for _, m := range mergedIds {
				if m == id[0] {
					rw.Write([]byte(")]}'\n[{\"submitted\":\"time\"}]"))
					return
				}
			}
			for _, u := range unmergedIds {
				if u == id[0] {
					rw.Write([]byte(fmt.Sprintf(")]}'\n[{\"change-id\":\"%s\"}]", id[0])))
					return
				}
			}
		}
		rw.Write([]byte(")]}'\n[]"))
	})
	serverMux.HandleFunc("/tools/hooks/commit-msg", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("#!/bin/sh"))
	})
	server := httptest.NewServer(serverMux)
	defer server.Close()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 6
	localProjects := createBranchProjects(t, fake, numProjects)

	m, err := fake.ReadRemoteManifest()
	if err != nil {
		t.Fatal(err)
	}
	ps := []project.Project{}
	for _, p := range m.Projects {
		p.GerritHost = server.URL
		ps = append(ps, p)
	}
	m.Projects = ps
	if err := fake.WriteRemoteManifest(m); err != nil {
		t.Fatal(err)
	}

	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	branchToDelete1 := "branchToDelete1"
	branchToDelete2 := "branchToDelete2"
	branchNotToDelete := "branchNotToDelete"

	changeIdPrefix := "Change-Id: "

	i := 0
	gitLocals[i].CreateBranch(branchToDelete1)
	gitLocals[i].CheckoutBranch(branchToDelete1, localProjects[i].GitSubmodules, false)
	for j := 0; j < 2; j++ {
		writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+mergedIds[j])
	}
	gitLocals[i].CreateBranchWithUpstream(branchToDelete2, "origin/main")
	gitLocals[i].CheckoutBranch(branchToDelete2, localProjects[i].GitSubmodules, false)
	for j := 0; j < 2; j++ {
		writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+mergedIds[j])
	}

	i = 1
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CheckoutBranch(branchToDelete1, localProjects[i].GitSubmodules, false)
	for j := 0; j < 2; j++ {
		writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+mergedIds[j])
	}
	gitLocals[i].CreateAndCheckoutBranch(branchNotToDelete)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile"+changeIdPrefix+localIds[0])

	i = 2
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+unmergedIds[0])

	// project-3 has no branch

	// Don't delete current branch with changes
	i = 4
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+mergedIds[0])
	newfile(t, localProjects[i].Path, "uncommitted.go")

	// Don't delete branch when it has local commits
	i = 5
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+localIds[0])
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile\n"+changeIdPrefix+mergedIds[0])

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}

	oldstates, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	got := executeBranch(t, fake, branchCmd{deleteMergedCLs: true})
	fmt.Println(got)

	newstates, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		oldstate, _ := oldstates[localProject.Key()]
		newstate, _ := newstates[localProject.Key()]
		newBranchMap := make(map[string]bool)
		for _, newb := range newstate.Branches {
			newBranchMap[newb.Name] = true
		}
		for _, oldb := range oldstate.Branches {
			if oldb.Name == branchNotToDelete && !newBranchMap[oldb.Name] {
				t.Errorf("project %q should contain branch %q", localProject.Name, oldb.Name)
			} else if strings.HasPrefix(oldb.Name, "branchToDelete") && newBranchMap[oldb.Name] {
				t.Errorf("project %q should not contain branch %q", localProject.Name, oldb.Name)
			}
		}
	}
}

func TestDeleteMergedBranch(t *testing.T) {
	t.Parallel()

	testDeleteMergedBranch(t, false)
	testDeleteMergedBranch(t, true)
}

func testDeleteMergedBranch(t *testing.T, overridePC bool) {
	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 7
	localProjects := createBranchProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	branchToDelete1 := "branchToDelete1"
	branchToDelete2 := "branchToDelete2"
	branchNotToDelete := "branchNotToDelete"

	i := 0
	gitLocals[i].CreateBranch(branchToDelete1)
	gitLocals[i].CreateBranchWithUpstream(branchToDelete2, "origin/main")

	i = 1
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")

	i = 2
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")

	// project-3 has no branch

	i = 4
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CreateBranch(branchToDelete2)
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	lc := project.LocalConfig{NoUpdate: true}
	project.WriteLocalConfig(fake.X, localProjects[i], lc)
	localProjects[i].LocalConfig = lc

	// Don't delete current branch with changes
	i = 5
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	newfile(t, localProjects[i].Path, "uncommitted.go")

	// Don't delete current branch with changes
	i = 6
	gitLocals[i].CreateBranch(branchToDelete1)
	gitLocals[i].CreateBranch(branchNotToDelete)
	gitLocals[i].CheckoutBranch(branchNotToDelete, localProjects[i].GitSubmodules, false)
	newfile(t, localProjects[i].Path, "uncommitted.go")

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}

	oldstates, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	executeBranch(t, fake, branchCmd{
		deleteMerged:          true,
		overrideProjectConfig: overridePC,
	})

	newstates, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		dontdelete := localProject.LocalConfig.NoUpdate && !overridePC
		oldstate, _ := oldstates[localProject.Key()]
		newstate, _ := newstates[localProject.Key()]
		newBranchMap := make(map[string]bool)
		for _, newb := range newstate.Branches {
			newBranchMap[newb.Name] = true
		}
		for _, oldb := range oldstate.Branches {
			if (dontdelete || oldb.Name == branchNotToDelete) && !newBranchMap[oldb.Name] {
				t.Errorf("project %q should contain branch %q", localProject.Name, oldb.Name)
			} else if !dontdelete && strings.HasPrefix(oldb.Name, "branchToDelete") && newBranchMap[oldb.Name] {
				t.Errorf("project %q should not contain branch %q", localProject.Name, oldb.Name)
			}
		}
	}

	// Test that if <branch> is passed only that branch is deleted
	i = 0
	gitLocals[i].CreateBranch(branchToDelete1)
	gitLocals[i].DeleteBranch(branchNotToDelete, gitutil.ForceOpt(true))
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")

	i = 1
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].DeleteBranch(branchNotToDelete, gitutil.ForceOpt(true))
	gitLocals[i].CreateBranchWithUpstream(branchNotToDelete, "origin/main")

	i = 2
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].DeleteBranch(branchNotToDelete, gitutil.ForceOpt(true))
	gitLocals[i].CreateBranch(branchNotToDelete)

	i = 3
	gitLocals[i].CreateBranch(branchToDelete1)
	gitLocals[i].DeleteBranch(branchNotToDelete, gitutil.ForceOpt(true))
	gitLocals[i].CreateBranch(branchNotToDelete)

	i = 4
	gitLocals[i].CreateBranchWithUpstream(branchToDelete1, "origin/main")
	gitLocals[i].DeleteBranch(branchNotToDelete, gitutil.ForceOpt(true))
	gitLocals[i].CreateBranch(branchNotToDelete)
	lc = project.LocalConfig{NoUpdate: true}
	project.WriteLocalConfig(fake.X, localProjects[i], lc)
	localProjects[i].LocalConfig = lc

	executeBranch(t, fake, branchCmd{
		deleteMerged:          true,
		overrideProjectConfig: overridePC,
	}, branchToDelete1)

	newstates, err = project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i <= 4; i++ {
		localProject := localProjects[i]
		dontdelete := localProject.LocalConfig.NoUpdate && !overridePC
		newstate, _ := newstates[localProject.Key()]
		newBranchMap := make(map[string]bool)
		for _, newb := range newstate.Branches {
			newBranchMap[newb.Name] = true
		}

		if !newBranchMap[branchNotToDelete] {
			t.Errorf("project %q should contain branch %q", localProject.Name, branchNotToDelete)
		}

		if dontdelete && !newBranchMap[branchToDelete1] {
			t.Errorf("project %q should contain branch %q", localProject.Name, branchToDelete1)
		}

		if !dontdelete && newBranchMap[branchToDelete1] {
			t.Errorf("project %q should not contain branch %q", localProject.Name, branchToDelete1)
		}
	}

}

func equalBranchOut(first, second string) bool {
	second = strings.TrimSpace(second)
	firstStrings := strings.Split(first, "\n")
	secondStrings := strings.Split(second, "\n")
	if len(firstStrings) != len(secondStrings) {
		return false
	}
	sort.Strings(firstStrings)
	sort.Strings(secondStrings)
	for i, first := range firstStrings {
		if first != secondStrings[i] {
			return false
		}
	}
	return true
}

func equalDefaultBranchOut(first, second string) bool {
	second = strings.TrimSpace(second)
	firstStrings := strings.Split(first, "\n\n")
	secondStrings := strings.Split(second, "\n\n")
	if len(firstStrings) != len(secondStrings) {
		return false
	}
	sort.Strings(firstStrings)
	sort.Strings(secondStrings)
	for i, first := range firstStrings {
		if first != secondStrings[i] {
			return false
		}
	}
	return true
}

func executeBranch(t *testing.T, fake *jiritest.FakeJiriRoot, c branchCmd, args ...string) string {
	stdout, stderr, err := collectStdio(fake.X, args, c.run)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.Join([]string{stdout, stderr}, " "))
}
