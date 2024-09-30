// Copyright 2024 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package integrationtests

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.fuchsia.dev/jiri/project"
)

func TestSimpleProject(t *testing.T) {
	t.Parallel()

	remoteDir := t.TempDir()
	setupGitRepo(t, remoteDir, map[string]any{
		"manifest": project.Manifest{
			Projects: []project.Project{
				{
					Name:   "manifest",
					Path:   "manifest_dir",
					Remote: remoteDir,
				},
			},
		},
	})

	root := t.TempDir()
	jiri := jiriInit(t, root)
	jiri("import", "manifest", remoteDir)
	jiri("update")

	wantFiles := []string{
		"manifest_dir/manifest",
	}

	gotFiles := listDirRecursive(t, root)
	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		t.Errorf("Wrong directory contents after update (-want +got):\n%s", diff)
	}
}

func TestUpdateWithDirtyProject(t *testing.T) {
	t.Parallel()

	remoteDir := t.TempDir()
	setupGitRepo(t, remoteDir, map[string]any{
		"manifest": project.Manifest{
			Projects: []project.Project{
				{
					Name:   "manifest",
					Path:   "manifest_dir",
					Remote: remoteDir,
				},
			},
		},
		"foo.txt": "original contents\n",
	})

	root := t.TempDir()
	jiri := jiriInit(t, root)
	jiri("import", "manifest", remoteDir)
	jiri("update")

	fooPath := filepath.Join(root, "manifest_dir", "foo.txt")
	newContents := "new contents\n"
	writeFile(t, fooPath, newContents)

	// A Jiri update should not discard uncommitted changes.
	jiri("update")

	got := readFile(t, fooPath)

	if diff := cmp.Diff(newContents, got); diff != "" {
		t.Errorf("Wrong foo.txt contents after update (-want +got):\n%s", diff)
	}
}

func TestUpdateWithProjectNotOnJIRI_HEAD(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (jiri func(args ...string) string, fooLocalDir string) {
		fooRemoteDir := t.TempDir()
		setupGitRepo(t, fooRemoteDir, map[string]any{"foo.txt": "foo"})

		// Commit bar.txt to the remote.
		writeFile(t, filepath.Join(fooRemoteDir, "bar.txt"), "bar\n")
		runSubprocess(t, fooRemoteDir, "git", "add", "bar.txt")
		runSubprocess(t, fooRemoteDir, "git", "commit", "-m", "Add bar.txt")

		remoteDir := t.TempDir()
		setupGitRepo(t, remoteDir, map[string]any{
			"manifest": project.Manifest{
				Projects: []project.Project{
					{
						Name:   "manifest",
						Path:   "manifest_dir",
						Remote: remoteDir,
					},
					{
						Name:   "foo",
						Path:   "foo",
						Remote: fooRemoteDir,
					},
				},
			},
			"foo.txt": "original contents\n",
		})

		root := t.TempDir()
		jiri = jiriInit(t, root)
		jiri("import", "manifest", remoteDir)
		jiri("update")

		fooLocalDir = filepath.Join(root, "foo")
		barPath := filepath.Join(fooLocalDir, "bar.txt")

		// Sanity check: Make sure bar.txt exists in the local checkout in the first
		// place.
		if !fileExists(t, barPath) {
			t.Fatalf("//foo/bar.txt not present after jiri update")
		}

		return jiri, fooLocalDir
	}

	t.Run("detached HEAD", func(t *testing.T) {
		t.Parallel()

		jiri, fooLocalDir := setup(t)
		barPath := filepath.Join(fooLocalDir, "bar.txt")

		// Check out the previous commit of the foo repo, putting the repo in a
		// "detached HEAD" state.
		runSubprocess(t, fooLocalDir, "git", "checkout", "HEAD^")

		// Sanity check: The file should not exist at the previous commit.
		if fileExists(t, barPath) {
			t.Fatalf("//foo/bar.txt still present after git checkout")
		}

		jiri("update")

		// The file should exist again after a jiri update - Jiri should
		// automatically reset the repository to JIRI_HEAD if it's in a detached
		// HEAD state.
		if !fileExists(t, barPath) {
			t.Errorf("jiri update didn't reset to JIRI_HEAD; //foo/bar.txt is not present")
		}
	})

	t.Run("on branch", func(t *testing.T) {
		t.Parallel()

		jiri, fooLocalDir := setup(t)
		barPath := filepath.Join(fooLocalDir, "bar.txt")

		// Check out the previous commit of the foo repo, *on a branch*.
		runSubprocess(t, fooLocalDir, "git", "checkout", "-b", "somebranch", "HEAD^")

		// Sanity check: The file should not exist at the previous commit.
		if fileExists(t, barPath) {
			t.Fatalf("//foo/bar.txt still present after git checkout")
		}

		runSubprocess(t, fooLocalDir, "git", "checkout", "-b", "temp")

		jiri("update")

		// The file should still NOT exist after a jiri update - Jiri should not
		// automatically reset the repository to JIRI_HEAD if it's on a branch.
		if fileExists(t, barPath) {
			t.Errorf("jiri update unexpectedly reset to JIRI_HEAD; //foo/bar.txt is present")
		}
	})
}

func TestUpdateWhileOnBranchEqualToJIRI_HEAD(t *testing.T) {
	fooRemoteDir := t.TempDir()
	setupGitRepo(t, fooRemoteDir, map[string]any{"foo.txt": "foo"})

	remoteDir := t.TempDir()
	setupGitRepo(t, remoteDir, map[string]any{
		"manifest": project.Manifest{
			Projects: []project.Project{
				{
					Name:   "manifest",
					Path:   "manifest_dir",
					Remote: remoteDir,
				},
				{
					Name:   "foo",
					Path:   "foo",
					Remote: fooRemoteDir,
				},
			},
		},
		"foo.txt": "original contents\n",
	})

	root := t.TempDir()
	jiri := jiriInit(t, root)
	jiri("import", "manifest", remoteDir)
	jiri("update")

	fooLocalDir := filepath.Join(root, "foo")
	barPath := filepath.Join(fooLocalDir, "bar.txt")

	// Sanity check: Make sure bar.txt does not exist in the first place.
	if fileExists(t, barPath) {
		t.Fatalf("//foo/bar.txt unexpectedly present")
	}

	// Commit bar.txt to the remote.
	writeFile(t, filepath.Join(fooRemoteDir, "bar.txt"), "bar\n")
	runSubprocess(t, fooRemoteDir, "git", "add", "bar.txt")
	runSubprocess(t, fooRemoteDir, "git", "commit", "-m", "Add bar.txt")

	// Create the main branch, which should automatically track the upstream
	// main branch - this is critical, because Jiri only automatically updates
	// branches that track remote branches.
	runSubprocess(t, fooLocalDir, "git", "checkout", "main")

	jiri("update")

	// Sanity check: Make sure bar.txt does not exist in the first place.
	if !fileExists(t, barPath) {
		t.Fatalf("//foo/bar.txt not present after jiri update, JIRI_HEAD should have advanced")
	}
}
