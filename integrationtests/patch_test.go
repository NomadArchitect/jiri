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

func TestPatch(t *testing.T) {
	t.Parallel()

	subprojectRemoteDir := t.TempDir()
	setupGitRepo(t, subprojectRemoteDir, map[string]any{"foo.txt": "foo"})

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
					Name:   "subproject",
					Path:   "subproject_dir",
					Remote: subprojectRemoteDir,
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
		"subproject_dir/foo.txt",
	}

	gotFiles := listDirRecursive(t, root)
	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		t.Errorf("Wrong directory contents after update (-want +got):\n%s", diff)
	}

	rebaseBranch := "base-branch"
	runSubprocess(t, subprojectRemoteDir, "git", "branch", rebaseBranch)

	patchBranch := "branch-to-patch"
	runSubprocess(t, subprojectRemoteDir, "git", "checkout", "-b", patchBranch)
	writeFile(t, filepath.Join(subprojectRemoteDir, "bar.txt"), "bar")
	runSubprocess(t, subprojectRemoteDir, "git", "add", "bar.txt")
	runSubprocess(t, subprojectRemoteDir, "git", "commit", "-m", "Add bar.txt")

	jiri("patch", "-project", "subproject", "-no-branch", "-rebase-branch", rebaseBranch, patchBranch)

	wantFiles = []string{
		"manifest_dir/manifest",
		"subproject_dir/bar.txt",
		"subproject_dir/foo.txt",
	}

	gotFiles = listDirRecursive(t, root)
	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		t.Errorf("Wrong directory contents after patch (-want +got):\n%s", diff)
	}
}
