// Copyright 2024 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package integrationtests

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.fuchsia.dev/jiri/cmd/jiri/subcommands"
	"go.fuchsia.dev/jiri/cmdline"
	"go.fuchsia.dev/jiri/envvar"
	"go.fuchsia.dev/jiri/project"
)

func runJiri(t *testing.T, root string, args ...string) string {
	t.Helper()

	args = append([]string{"--root", root}, args...)

	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{
		Stdout: &stdout,
		Stderr: &stderr,
		Vars:   envvar.SliceToMap(os.Environ()),
	}
	err := cmdline.ParseAndRun(subcommands.NewCmdRoot(), env, args)
	if err != nil {
		t.Fatalf("%q failed: %s\n%s",
			strings.Join(append([]string{"jiri"}, args...), " "),
			err,
			string(stderr.Bytes()),
		)
	}
	return string(stdout.Bytes())
}

func runSubprocess(t *testing.T, dir string, args ...string) string {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"GIT_CONFIG_COUNT=2",
		// Allow adding local git directories as submodules.
		"GIT_CONFIG_KEY_0=protocol.file.allow", "GIT_CONFIG_VALUE_0=always",
		"GIT_CONFIG_KEY_1=init.defaultbranch", "GIT_CONFIG_VALUE_1=main",
	)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		cmdline := strings.Join(args, " ")
		msg := stderr.String()
		if msg == "" && stdout.String() != "" {
			msg = stdout.String()
		}
		t.Fatalf("%q failed: %s\n%s", cmdline, err, msg)
	}
	return string(stdout.Bytes())
}

func writeFile(t *testing.T, path string, contents []byte) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSimpleProject(t *testing.T) {
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
	runJiri(t, root, "init", "-analytics-opt=false", root)
	runJiri(t, root, "import", "manifest", remoteDir)
	runJiri(t, root, "update")

	t.Log(runSubprocess(t, root, "tree"))
}

func TestSubmodules(t *testing.T) {
	remoteDir := t.TempDir()
	setupGitRepo(t, remoteDir, map[string]any{
		"manifest": project.Manifest{
			Projects: []project.Project{
				{
					Name:          "manifest",
					Path:          "manifest_dir",
					Remote:        remoteDir,
					GitSubmodules: true,
				},
			},
		},
	})

	submoduleRemoteDir := t.TempDir()
	setupGitRepo(t, submoduleRemoteDir, map[string]any{
		"foo.txt": "foo",
	})

	runSubprocess(t, remoteDir, "git", "submodule", "add", submoduleRemoteDir, "submodule")
	runSubprocess(t, remoteDir, "git", "commit", "-m", "Add submodule")

	root := t.TempDir()
	runJiri(t, root, "init", "-analytics-opt=false", "-enable-submodules=true", root)
	runJiri(t, root, "import", "manifest", remoteDir)
	runJiri(t, root, "update")

	// TODO: this is the bit that causes issues.
	runSubprocess(t, filepath.Join(root, "manifest_dir"), "git", "checkout", "main")

	// Create a new commit in the submodule's upstream.
	writeFile(t, filepath.Join(submoduleRemoteDir, "bar.txt"), []byte("bar"))
	runSubprocess(t, submoduleRemoteDir, "git", "add", ".")
	runSubprocess(t, submoduleRemoteDir, "git", "commit", "-m", "Add bar.txt")

	// Sync the submodule to its upstream.
	runSubprocess(t, filepath.Join(remoteDir, "submodule"), "git", "pull")
	runSubprocess(t, remoteDir, "git", "commit", "-a", "-m", "Update submodule")

	// jiri update
	runJiri(t, root, "update")

	t.Log(runSubprocess(t, root, "tree"))
}

func setupGitRepo(t *testing.T, dir string, files map[string]any) {
	t.Helper()

	runSubprocess(t, dir, "git", "init")

	for path, contents := range files {
		var b []byte
		switch x := contents.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		case project.Manifest:
			var err error
			b, err = x.ToBytes()
			if err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("Invalid type for git repo file %s", path)
		}

		writeFile(t, filepath.Join(dir, path), b)
	}

	runSubprocess(t, dir, "git", "add", ".")
	runSubprocess(t, dir, "git", "commit", "-m", "Initial commit")
}
