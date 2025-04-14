// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subcommands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/gitutil"
	"go.fuchsia.dev/jiri/jiritest"
	"go.fuchsia.dev/jiri/project"
	"go.fuchsia.dev/jiri/tool"
)

func checkReadme(t *testing.T, jirix *jiri.X, project, message string) {
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("%v", err)
	}
	readmeFile := filepath.Join(project, "README")
	data, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got, want := data, []byte(message); bytes.Compare(got, want) != 0 {
		t.Fatalf("unexpected content %v:\ngot\n%s\nwant\n%s\n", project, got, want)
	}
}

func localProjectName(i int) string {
	return "test-local-project-" + fmt.Sprintf("%d", i+1)
}

func remoteProjectName(i int) string {
	return "test-remote-project-" + fmt.Sprintf("%d", i+1)
}

func writeReadme(t *testing.T, jirix *jiri.X, projectDir, message string) {
	path, perm := filepath.Join(projectDir, "README"), os.FileMode(0644)
	if err := os.WriteFile(path, []byte(message), perm); err != nil {
		t.Fatalf("%s", err)
	}
	if err := gitutil.New(jirix, gitutil.RootDirOpt(projectDir), gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com")).CommitFile(path, "creating README"); err != nil {
		t.Fatalf("%s", err)
	}
}

// TestSnapshot tests creating and checking out a snapshot.
func TestSnapshot(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Setup the initial remote and local projects.
	numProjects, remoteProjects := 2, []string{}
	for i := 0; i < numProjects; i++ {
		if err := fake.CreateRemoteProject(remoteProjectName(i)); err != nil {
			t.Fatalf("%v", err)
		}
		if err := fake.AddProject(project.Project{
			Name:   remoteProjectName(i),
			Path:   localProjectName(i),
			Remote: fake.Projects[remoteProjectName(i)],
		}); err != nil {
			t.Fatalf("%v", err)
		}
	}

	// Create initial commits in the remote projects and use UpdateUniverse()
	// to mirror them locally.
	for i := 0; i < numProjects; i++ {
		writeReadme(t, fake.X, fake.Projects[remoteProjectName(i)], "revision 1")
	}
	if err := project.UpdateUniverse(fake.X, project.UpdateUniverseParams{
		GC:                   true,
		RunHooks:             true,
		FetchPackages:        true,
		RunHookTimeout:       project.DefaultHookTimeout,
		FetchPackagesTimeout: project.DefaultPackageTimeout,
	}); err != nil {
		t.Fatalf("%v", err)
	}

	// Create a snapshot.
	var stdout bytes.Buffer
	fake.X.Context = tool.NewContext(tool.ContextOpts{Stdout: &stdout, Env: fake.X.Context.Env()})

	tmpfile, err := os.CreateTemp("", "jiri-snapshot-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if err := (&snapshotCmd{}).run(fake.X, []string{tmpfile.Name()}); err != nil {
		t.Fatalf("%v", err)
	}

	// Remove the local project repositories.
	for i := range remoteProjects {
		localProject := filepath.Join(fake.X.Root, localProjectName(i))
		if err := os.RemoveAll(localProject); err != nil {
			t.Fatalf("%v", err)
		}
	}

	snapshotFile := tmpfile.Name()
	if err := project.CheckoutSnapshot(fake.X, snapshotFile, false, true /*run-hooks*/, true /*run-packages*/, project.DefaultHookTimeout, project.DefaultPackageTimeout, nil); err != nil {
		t.Fatalf("%s", err)
	}
	for i := range remoteProjects {
		localProject := filepath.Join(fake.X.Root, localProjectName(i))
		checkReadme(t, fake.X, localProject, "revision 1")
	}
}

// TestCipdSnapshot tests creating cipd snapshot files.
func TestCipdSnapshot(t *testing.T) {
	t.Parallel()

	fake := jiritest.NewFakeJiriRoot(t)

	// Setup fake packages
	fake.AddPackage(project.Package{
		Name:       "test_package",
		Path:       "path-to-test-package",
		Version:    "git_revision:05715c8fbbdb952ab38e50533a1b653445e74b40",
		Attributes: "",
	})
	fake.AddPackage(project.Package{
		Name:       "test_package_internal",
		Path:       "path-to-test-package-internal",
		Version:    "git_revision:05715c8fbbdb952ab38e50533a1b653445e74b41",
		Attributes: "",
		Internal:   true,
	})

	// Create a snapshot.
	var stdout bytes.Buffer
	fake.X.Context = tool.NewContext(tool.ContextOpts{Stdout: &stdout, Env: fake.X.Context.Env()})

	tmpfile, err := os.CreateTemp("", "jiri-snapshot-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if err := project.CreateSnapshot(fake.X, tmpfile.Name(), nil, nil, true /*cipdEnsureFlag*/, nil); err != nil {
		t.Fatalf("%v", err)
	}
	pathExists := func(pkgPath string) bool {
		if _, err := os.Stat(pkgPath); err != nil {
			if os.IsNotExist(err) {
				return false
			}
			t.Errorf("failed to access path due to error: %v", err)
		}
		return true
	}
	assertExist := func(localPath string) {
		if !pathExists(localPath) {
			t.Errorf("expecting path %q exists, but it does not", localPath)
		}
	}

	// Verify cipd snapshot files were generated.
	ensureFilePath := tmpfile.Name() + ".ensure"
	ensureFileIntPath := tmpfile.Name() + "_internal.ensure"
	versionFilePath := tmpfile.Name() + ".version"
	versionFileIntPath := tmpfile.Name() + "_internal.version"
	assertExist(ensureFilePath)
	assertExist(ensureFileIntPath)
	assertExist(versionFilePath)
	assertExist(versionFileIntPath)
	os.Remove(ensureFilePath)
	os.Remove(ensureFileIntPath)
	os.Remove(versionFilePath)
	os.Remove(versionFileIntPath)
}
