// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subcommands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/gitutil"
	"go.fuchsia.dev/jiri/jiritest"
	"go.fuchsia.dev/jiri/project"
)

func defaultStatusFlags() *statusCmd {
	return &statusCmd{
		changes:   true,
		checkHead: true,
		commits:   true,
	}
}

func createCommits(t *testing.T, fake *jiritest.FakeJiriRoot, localProjects []project.Project) ([]string, []string, []string, []string) {
	var file2CommitRevs []string
	var file1CommitRevs []string
	var latestCommitRevs []string
	var relativePaths []string
	for i, localProject := range localProjects {
		setDummyUser(t, fake.X, fake.Projects[localProject.Name])
		gitRemote := gitutil.New(fake.X, gitutil.RootDirOpt(fake.Projects[localProject.Name]))
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file1"+strconv.Itoa(i), "file1"+strconv.Itoa(i))
		gitRemote.CreateAndCheckoutBranch("file-1")
		gitRemote.Checkout("main")
		file1CommitRev, _ := gitRemote.CurrentRevision()
		file1CommitRevs = append(file1CommitRevs, file1CommitRev)
		gitRemote.CreateAndCheckoutBranch("file-2")
		gitRemote.Checkout("main")
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file2"+strconv.Itoa(i), "file2"+strconv.Itoa(i))
		file2CommitRev, _ := gitRemote.CurrentRevision()
		file2CommitRevs = append(file2CommitRevs, file2CommitRev)
		writeFile(t, fake.X, fake.Projects[localProject.Name], "file3"+strconv.Itoa(i), "file3"+strconv.Itoa(i))
		file3CommitRev, _ := gitRemote.CurrentRevision()
		latestCommitRevs = append(latestCommitRevs, file3CommitRev)
		relativePath, _ := filepath.Rel(fake.X.Cwd, localProject.Path)
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

func expectedOutput(t *testing.T, fake *jiritest.FakeJiriRoot, cmd *statusCmd, localProjects []project.Project,
	latestCommitRevs, currentCommits, changes, currentBranch, relativePaths []string, extraCommitLogs [][]string) string {
	want := ""
	for i, localProject := range localProjects {
		includeForNotHead := cmd.checkHead && currentCommits[i] != latestCommitRevs[i]
		includeForChanges := cmd.changes && changes[i] != ""
		includeForCommits := cmd.commits && extraCommitLogs != nil && len(extraCommitLogs[i]) != 0
		includeProject := (cmd.branch == "" && (includeForNotHead || includeForChanges || includeForCommits)) ||
			(cmd.branch != "" && cmd.branch == currentBranch[i])
		if includeProject {
			gitLocal := gitutil.New(fake.X, gitutil.RootDirOpt(localProject.Path))
			currentLog, err := gitLocal.OneLineLog(currentCommits[i])
			if err != nil {
				t.Error(err)
			}
			want = fmt.Sprintf("%s%s: ", want, relativePaths[i])
			if currentCommits[i] != latestCommitRevs[i] && cmd.checkHead {
				log, err := gitLocal.OneLineLog(latestCommitRevs[i])
				if err != nil {
					t.Error(err)
				}
				want = fmt.Sprintf("%s\nJIRI_HEAD: %s", want, log)
				want = fmt.Sprintf("%s\nCurrent Revision: %s", want, currentLog)
			}
			want = fmt.Sprintf("%s\nBranch: ", want)
			branchmsg := currentBranch[i]
			if branchmsg == "" {
				branchmsg = fmt.Sprintf("DETACHED-HEAD(%s)", currentLog)
			}
			want = fmt.Sprintf("%s%s", want, branchmsg)
			if extraCommitLogs != nil && cmd.commits && len(extraCommitLogs[i]) != 0 {
				want = fmt.Sprintf("%s\nCommits: %d commit(s) not merged to remote", want, len(extraCommitLogs[i]))
				for _, commitLog := range extraCommitLogs[i] {
					want = fmt.Sprintf("%s\n%s", want, commitLog)
				}

			}
			if cmd.changes && changes[i] != "" {
				want = fmt.Sprintf("%s\n%s", want, changes[i])
			}
			want = fmt.Sprintf("%s\n\n", want)
		}
	}
	want = strings.TrimSpace(want)
	return want
}

func TestStatus(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 3
	localProjects := createProjects(t, fake, numProjects)
	file1CommitRevs, file2CommitRevs, latestCommitRevs, relativePaths := createCommits(t, fake, localProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	for _, lp := range localProjects {
		setDummyUser(t, fake.X, lp.Path)
	}
	// Test no changes
	cmd := defaultStatusFlags()
	got := executeStatus(t, fake, cmd, "")
	want := ""
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// Test when HEAD is on different revsion
	gitLocal := gitutil.New(fake.X, gitutil.RootDirOpt(localProjects[1].Path))
	gitLocal.Checkout("HEAD~1")
	gitLocal = gitutil.New(fake.X, gitutil.RootDirOpt(localProjects[2].Path))
	gitLocal.Checkout("file-2")
	got = executeStatus(t, fake, cmd, "")
	currentCommits := []string{latestCommitRevs[0], file2CommitRevs[1], file1CommitRevs[2]}
	currentBranch := []string{"", "", "file-2"}
	changes := []string{"", "", ""}
	want = expectedOutput(t, fake, cmd, localProjects, latestCommitRevs, currentCommits, changes, currentBranch, relativePaths, nil)
	if !equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}

	// Test combinations of tracked and untracked changes
	newfile(t, localProjects[0].Path, "untracked1")
	newfile(t, localProjects[0].Path, "untracked2")
	newfile(t, localProjects[2].Path, "uncommitted.go")
	if err := gitLocal.Add("uncommitted.go"); err != nil {
		t.Error(err)
	}
	got = executeStatus(t, fake, cmd, "")
	currentCommits = []string{latestCommitRevs[0], file2CommitRevs[1], file1CommitRevs[2]}
	currentBranch = []string{"", "", "file-2"}
	changes = []string{"?? untracked1\n?? untracked2", "", "A  uncommitted.go"}
	want = expectedOutput(t, fake, cmd, localProjects, latestCommitRevs, currentCommits, changes, currentBranch, relativePaths, nil)
	if !equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestStatusWhenUserUpdatesGitTree(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 1
	localProjects := createProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	// write to remote
	writeFile(t, fake.X, fake.Projects[localProjects[0].Name], "file", "file")
	// git fetch
	gitLocal := gitutil.New(fake.X, gitutil.RootDirOpt(localProjects[0].Path))
	if err := gitLocal.Fetch("origin"); err != nil {
		t.Fatal(err)
	}

	cmd := defaultStatusFlags()
	got := executeStatus(t, fake, cmd, "")
	want := "" // no change
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestStatusDeleted(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 5
	createProjects(t, fake, numProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	// delete some projects
	manifest, err := fake.ReadRemoteManifest()
	if err != nil {
		t.Fatal(err)
	}
	deletedProjs := manifest.Projects[3:]
	manifest.Projects = manifest.Projects[0:3]
	if err := fake.WriteRemoteManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	cmd := defaultStatusFlags()
	cmd.deleted = true

	got := executeStatus(t, fake, cmd, "")
	numOfLines := len(strings.Split(got, "\n"))
	if numOfLines != 3 {
		t.Errorf("got %s, wanted 3 deleted projects", got)
	}
	for _, dp := range deletedProjs {
		if !strings.Contains(got, dp.Name) {
			t.Fatalf("project %s should have been deleted, got\n%s", dp.Name, got)
		}
		if !strings.Contains(got, dp.Path) {
			t.Fatalf("project %s should have been deleted, got\n%s", dp.Path, got)
		}
	}
}

func statusFlagsTest(t *testing.T, cmd *statusCmd) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Add projects
	numProjects := 6
	localProjects := createProjects(t, fake, numProjects)
	file1CommitRevs, file2CommitRevs, latestCommitRevs, relativePaths := createCommits(t, fake, localProjects)
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}
	gitLocals := make([]*gitutil.Git, numProjects)
	for i, localProject := range localProjects {
		gitLocal := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(localProject.Path))
		gitLocals[i] = gitLocal
	}

	gitLocals[0].Checkout("HEAD~1")
	gitLocals[1].Checkout("file-2")
	gitLocals[3].Checkout("HEAD~2")
	gitLocals[4].Checkout("main")
	gitLocals[5].Checkout("main")

	newfile(t, localProjects[0].Path, "untracked1")
	newfile(t, localProjects[0].Path, "untracked2")

	newfile(t, localProjects[1].Path, "uncommitted.go")
	if err := gitLocals[1].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}

	newfile(t, localProjects[2].Path, "untracked1")
	newfile(t, localProjects[2].Path, "uncommitted.go")
	if err := gitLocals[2].Add("uncommitted.go"); err != nil {
		t.Error(err)
	}

	extraCommits5 := []string{}
	for i := 0; i < 2; i++ {
		file := fmt.Sprintf("extrafile%d", i)
		writeFile(t, fake.X, localProjects[5].Path, file, file+"log")
		log, err := gitLocals[5].OneLineLog("HEAD")
		if err != nil {
			t.Error(err)
		}
		extraCommits5 = append([]string{log}, extraCommits5...)
	}
	gl5 := gitutil.New(fake.X, gitutil.RootDirOpt(localProjects[5].Path))
	currentCommit5, err := gl5.CurrentRevision()
	if err != nil {
		t.Error(err)
	}
	got := executeStatus(t, fake, cmd, "")
	currentCommits := []string{file2CommitRevs[0], file1CommitRevs[1], latestCommitRevs[2], file1CommitRevs[3], latestCommitRevs[4], currentCommit5}
	extraCommitLogs := [][]string{nil, nil, nil, nil, nil, extraCommits5}
	currentBranch := []string{"", "file-2", "", "", "main", "main"}
	changes := []string{"?? untracked1\n?? untracked2", "A  uncommitted.go", "A  uncommitted.go\n?? untracked1", "", "", ""}
	want := expectedOutput(t, fake, cmd, localProjects, latestCommitRevs, currentCommits, changes, currentBranch, relativePaths, extraCommitLogs)
	if !equal(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestStatusFlags(t *testing.T) {
	t.Parallel()

	t.Run("default flags", func(t *testing.T) {
		statusFlagsTest(t, defaultStatusFlags())
	})

	t.Run("no changes", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		statusFlagsTest(t, cmd)
	})

	t.Run("no changes, no check head", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		cmd.checkHead = false
		statusFlagsTest(t, cmd)
	})

	t.Run("no check head", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.checkHead = false
		statusFlagsTest(t, cmd)
	})

	t.Run("no changes, no check head, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		cmd.checkHead = false
		cmd.branch = "main"
		statusFlagsTest(t, cmd)
	})

	t.Run("no check head, no commits, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.checkHead = false
		cmd.branch = "main"
		cmd.commits = false
		statusFlagsTest(t, cmd)
	})

	t.Run("no changes, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		cmd.branch = "main"
		statusFlagsTest(t, cmd)
	})

	t.Run("no changes, no check head, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		cmd.checkHead = false
		cmd.branch = "file-2"
		statusFlagsTest(t, cmd)
	})

	t.Run("no check head, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.checkHead = false
		cmd.branch = "file-2"
		statusFlagsTest(t, cmd)
	})

	t.Run("no changes, branch", func(t *testing.T) {
		cmd := defaultStatusFlags()
		cmd.changes = false
		cmd.branch = "file-2"
		statusFlagsTest(t, cmd)
	})
}

func equal(first, second string) bool {
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

func executeStatus(t *testing.T, fake *jiritest.FakeJiriRoot, cmd *statusCmd, args ...string) string {
	stdout, _, err := collectStdio(fake.X, args, cmd.run)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.Join([]string{stdout}, " "))
}

func writeFile(t *testing.T, jirix *jiri.X, projectDir, fileName, message string) {
	path, perm := filepath.Join(projectDir, fileName), os.FileMode(0644)
	if err := os.WriteFile(path, []byte(message), perm); err != nil {
		t.Fatalf("WriteFile(%s, %d) failed: %s", path, perm, err)
	}
	if err := gitutil.New(jirix, gitutil.RootDirOpt(projectDir),
		gitutil.UserNameOpt("John Doe"),
		gitutil.UserEmailOpt("john.doe@example.com")).CommitFile(path,
		message); err != nil {
		t.Fatal(err)
	}
}

func setDummyUser(t *testing.T, jirix *jiri.X, projectDir string) {
	git := gitutil.New(jirix, gitutil.RootDirOpt(projectDir))
	if err := git.Config("user.email", "john.doe@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := git.Config("user.name", "John Doe"); err != nil {
		t.Fatal(err)
	}
}

func newfile(t *testing.T, dir, file string) {
	testfile := filepath.Join(dir, file)
	_, err := os.Create(testfile)
	if err != nil {
		t.Errorf("failed to create %s: %s", testfile, err)
	}
}
