// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"fuchsia.googlesource.com/jiri/color"
	"fuchsia.googlesource.com/jiri/gitutil"
	"fuchsia.googlesource.com/jiri/jiritest"
	"fuchsia.googlesource.com/jiri/project"
)

func setDefaultDeleteBranchFlags() {
	deleteBranchFlags.deleteFlag = false
	deleteBranchFlags.branchFlag = ""
}

func createCommits(t *testing.T, fake *jiritest.FakeJiriRoot, localProjects []project.Project) ([]string, []string, []string, []string) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var file2CommitRevs []string
	var file1CommitRevs []string
	var latestCommitRevs []string
	var relativePaths []string
	s := fake.X.NewSeq()
	for i, localProject := range localProjects {
		gitRemote := gitutil.New(s, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(fake.Projects[localProject.Name]))
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file1"+strconv.Itoa(i), "file1"+strconv.Itoa(i))
		gitRemote.CreateAndCheckoutBranch("file-1")
		gitRemote.CheckoutBranch("master")
		file1CommitRev, _ := gitRemote.CurrentRevision()
		file1CommitRevs = append(file1CommitRevs, file1CommitRev)
		gitRemote.CreateAndCheckoutBranch("file-2")
		gitRemote.CheckoutBranch("master")
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file2"+strconv.Itoa(i), "file2"+strconv.Itoa(i))
		file2CommitRev, _ := gitRemote.CurrentRevision()
		file2CommitRevs = append(file2CommitRevs, file2CommitRev)
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file3"+strconv.Itoa(i), "file3"+strconv.Itoa(i))
		file3CommitRev, _ := gitRemote.CurrentRevision()
		latestCommitRevs = append(latestCommitRevs, file3CommitRev)
		relativePath, _ := filepath.Rel(cwd, localProject.Path)
		relativePaths = append(relativePaths, relativePath)
	}
	return file1CommitRevs, file2CommitRevs, latestCommitRevs, relativePaths
}

func createProjects(t *testing.T, fake *jiritest.FakeJiriRoot, numProjects int) []project.Project {
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
	return localProjects
}

func TestDeleteBranchWithDeleteFlag(t *testing.T) {
	setDefaultDeleteBranchFlags()
	color.ColorFlag = false
	fake, cleanup := jiritest.NewFakeJiriRoot(t)
	defer cleanup()
	s := fake.X.NewSeq()

	// Add projects
	numProjects := 10
	localProjects := createProjects(t, fake, numProjects)
	file1CommitRevs, file2CommitRevs, latestCommitRevs, relativePaths := createCommits(t, fake, localProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(s, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	// Test no changes
	want := ""
	newfile := func(dir, file string) {
		testfile := filepath.Join(dir, file)
		_, err := s.Create(testfile)
		if err != nil {
			t.Errorf("failed to create %s: %v", testfile, err)
		}
	}
	testBranch := "testBranch"
	deleteBranchFlags.branchFlag = testBranch
	deleteBranchFlags.deleteFlag = true

	// Test case when new test branch is on HEAD
	i := 0
	gitLocals[i].CreateBranch(testBranch)
	want = fmt.Sprintf("%vProject %v(%v): Branch deleted", want, localProjects[i].Name, relativePaths[i])

	// Project-1 doesn't contain test branch

	// Test when test branch is on some previous commit
	i = 2
	gitLocals[i].CreateBranchFromCommit(testBranch, file2CommitRevs[i])
	want = fmt.Sprintf("%v\nProject %v(%v): Branch deleted", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is on that branch
	i = 3
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	want = fmt.Sprintf("%v\nProject %v(%v): Branch deleted. It might have left some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is project is on HEAD
	i = 4
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	gitLocals[i].CheckoutBranch(latestCommitRevs[i])
	want = fmt.Sprintf("%v\nProject %v(%v): Branch deleted. It might have left some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is on that branch
	i = 5
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	want = fmt.Sprintf("%v\nProject %v(%v): Branch deleted. It might have left some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch contain uncommited changes
	i = 6
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Has uncommited changes, will not delete it", want, localProjects[i].Name, relativePaths[i])

	// Test when different branch contain uncommited changes
	i = 7
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Branch deleted", want, localProjects[i].Name, relativePaths[i])

	// Test when branch contain uncommited changes and also some extra commits
	i = 8
	gitLocals[i].CreateBranchFromCommit(testBranch, file2CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Has uncommited changes, will not delete it", want, localProjects[i].Name, relativePaths[i])

	want = fmt.Sprintf("%v\nPlease check warnings above", want)

	got := executeDeleteBranch(t, fake, "")
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}
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
		if i == 6 || i == 8 {
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		} else if branchFound {
			t.Errorf("project %q should NOT contain branch %q", localProject.Name, testBranch)
		}

		if i == 6 || i == 8 {
			if state.CurrentBranch.Name != testBranch {
				t.Errorf("project %q should be on branch %q", localProject.Name, testBranch)
			}
		} else {
			if state.CurrentBranch.Name != "" {
				t.Errorf("project %q should be on detached head", localProject.Name)
			}
			if state.CurrentBranch.Revision != latestCommitRevs[i] {
				t.Errorf("project %q should be on revision %q", localProject.Name, latestCommitRevs[i])
			}
		}

	}
}

func TestDeleteBranchWithoutDeleteFlag(t *testing.T) {
	setDefaultDeleteBranchFlags()
	color.ColorFlag = false
	fake, cleanup := jiritest.NewFakeJiriRoot(t)
	defer cleanup()
	s := fake.X.NewSeq()

	// Add projects
	numProjects := 10
	localProjects := createProjects(t, fake, numProjects)
	file1CommitRevs, file2CommitRevs, latestCommitRevs, relativePaths := createCommits(t, fake, localProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(s, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	// Test no changes
	want := ""
	newfile := func(dir, file string) {
		testfile := filepath.Join(dir, file)
		_, err := s.Create(testfile)
		if err != nil {
			t.Errorf("failed to create %s: %v", testfile, err)
		}
	}
	testBranch := "testBranch"
	deleteBranchFlags.branchFlag = testBranch

	// Test case when new test branch is on HEAD
	i := 0
	gitLocals[i].CreateBranch(testBranch)
	want = fmt.Sprintf("%vProject %v(%v): Clean branch deletion", want, localProjects[i].Name, relativePaths[i])

	// Project-1 doesn't contain test branch

	// Test when test branch is on some previous commit
	i = 2
	gitLocals[i].CreateBranchFromCommit(testBranch, file2CommitRevs[i])
	want = fmt.Sprintf("%v\nProject %v(%v): Clean branch deletion", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is on that branch
	i = 3
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	want = fmt.Sprintf("%v\nProject %v(%v): Branch is not merged to origin. It may leave some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is project is on HEAD
	i = 4
	gitLocals[i].CreateBranch(testBranch)
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	gitLocals[i].CheckoutBranch(latestCommitRevs[i])
	want = fmt.Sprintf("%v\nProject %v(%v): Branch is not merged to origin. It may leave some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch on latest with extra commit and is on that branch
	i = 5
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	want = fmt.Sprintf("%v\nProject %v(%v): Branch is not merged to origin. It may leave some dangling commits behind", want, localProjects[i].Name, relativePaths[i])

	// Test when test branch contain uncommited changes
	i = 6
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Has uncommited changes, will not delete it", want, localProjects[i].Name, relativePaths[i])

	// Test when different branch contain uncommited changes
	i = 7
	gitLocals[i].CreateBranchFromCommit(testBranch, file1CommitRevs[i])
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Clean branch deletion", want, localProjects[i].Name, relativePaths[i])

	// Test when branch contain uncommited changes and also some extra commits
	i = 8
	gitLocals[i].CreateBranchFromCommit(testBranch, file2CommitRevs[i])
	gitLocals[i].CheckoutBranch(testBranch)
	writeFile(t, fake.X, localProjects[i].Path, "extrafile", "extrafile")
	newfile(localProjects[i].Path, "uncommitted.go")
	if err := gitLocals[i].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	want = fmt.Sprintf("%v\nProject %v(%v): Has uncommited changes, will not delete it", want, localProjects[i].Name, relativePaths[i])

	want = fmt.Sprintf("%v\nPlease check warnings above", want)
	want = fmt.Sprintf("%v\nNo branches were deleted, please run with -delete flag", want)

	got := executeDeleteBranch(t, fake, "")
	if !equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	projects := make(project.Projects)
	for _, localProject := range localProjects {
		projects[localProject.Key()] = localProject
	}
	states, err := project.GetProjectStates(fake.X, projects, false)
	if err != nil {
		t.Error(err)
	}

	// test project states
	for i = 0; i < numProjects; i++ {
		localProject := localProjects[i]
		state, _ := states[localProject.Key()]
		if i != 1 && i != 9 {
			branchFound := false
			for _, branch := range state.Branches {
				if branch.Name == testBranch {
					branchFound = true
					break
				}
			}
			if !branchFound {
				t.Errorf("project %q should contain branch %q", localProject.Name, testBranch)
			}
		}

		if i == 3 || i == 6 || i == 5 || i == 8 {
			if state.CurrentBranch.Name != testBranch {
				t.Errorf("project %q should be on branch %q", localProject.Name, testBranch)
			}
		} else {
			if state.CurrentBranch.Name != "" {
				t.Errorf("project %q should be on detached head", localProject.Name)
			}
			if state.CurrentBranch.Revision != latestCommitRevs[i] {
				t.Errorf("project %q should be on revision %q", localProject.Name, latestCommitRevs[i])
			}
		}

	}
}

func equal(first, second string) bool {
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

func executeDeleteBranch(t *testing.T, fake *jiritest.FakeJiriRoot, args ...string) string {
	stderr := ""
	runCmd := func() {
		if err := runDeleteBranch(fake.X, args); err != nil {
			stderr = err.Error()
		}
	}
	stdout, _, err := runfunc(runCmd)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.Join([]string{stdout, stderr}, " "))
}
