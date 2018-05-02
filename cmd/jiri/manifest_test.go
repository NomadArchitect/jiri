// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"path/filepath"
	"testing"

	"fuchsia.googlesource.com/jiri/jiritest"
)

func TestReadManifest(t *testing.T) {
	// Store the path to the test manifest file.
	relManifestPath := "../../cmdline/testdata/test_manifest"
	absManifestPath, err := filepath.Abs(relManifestPath)
	if err != nil {
		t.Fatal("missing test manifest file at " + relManifestPath)
	}

	runCommand := func(t *testing.T, args []string) (stdout string, stderr string) {
		// Set up a fake Jiri root to pass to our command.
		fake, cleanup := jiritest.NewFakeJiriRoot(t)
		defer cleanup()

		// Initialize flags for the command.
		flagSet := flag.NewFlagSet("read-manifest-test", flag.ContinueOnError)
		setManifestFlags(flagSet)
		flagSet.Parse(args)

		// Run the command.
		runCmd := func() {
			if err := runManifest(fake.X, flagSet.Args()); err != nil {
				// Capture the error as stderr since Jiri subcommands don't
				// intenionally print to stderr when they fail.
				stderr = err.Error()
			}
		}

		var err error
		stdout, _, err = runfunc(runCmd)
		if err != nil {
			t.Fatal(err)
		}

		return stdout, stderr
	}

	// Expects runReadManifest to return a specific value when given args.
	expectAttributeValue := func(t *testing.T, args []string, expectedValue string) {
		stdout, stderr := runCommand(t, args)

		// If an error occurred, fail.
		if stderr != "" {
			t.Error("error:", stderr)
			return
		}

		// Compare stdout to the expected value.
		if stdout != expectedValue {
			t.Errorf("expected %s, got %s", expectedValue, stdout)
		}
	}

	// Expects runReadManifest to error when given args.
	expectError := func(t *testing.T, args []string) {
		stdout, stderr := runCommand(t, args)

		// Fail if no error was output.
		if stderr == "" {
			t.Errorf("expected an error, got %s", stdout)
			return
		}
	}

	t.Run("should fail if manifest file is missing", func(t *testing.T) {
		expectError(t, []string{
			"-element=the_import",
			"-attribute=name",
		})

		expectError(t, []string{
			"-element=the_project",
			"-attribute=name",
		})
	})

	t.Run("should fail if -attribute is missing", func(t *testing.T) {
		expectError(t, []string{
			"-element=the_import",
			absManifestPath,
		})

		expectError(t, []string{
			"-element=the_project",
			absManifestPath,
		})
	})

	t.Run("should fail if -element is missing", func(t *testing.T) {
		expectError(t, []string{
			"-attribute=name",
			absManifestPath,
		})

		expectError(t, []string{
			"-attribute=name",
			absManifestPath,
		})
	})

	t.Run("should read <project> attributes", func(t *testing.T) {
		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=name",
			absManifestPath,
		},
			"the_project")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=remote",
			absManifestPath,
		},
			"https://fuchsia.googlesource.com/the_project")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=revision",
			absManifestPath,
		},
			"the_project_revision")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=remotebranch",
			absManifestPath,
		},
			"the_project_remotebranch")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=path",
			absManifestPath,
		},
			"path/to/the_project")
	})

	t.Run("should read <import> attributes", func(t *testing.T) {
		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=name",
			absManifestPath,
		},
			"the_import")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=remote",
			absManifestPath,
		},
			"https://fuchsia.googlesource.com/the_import")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=manifest",
			absManifestPath,
		},
			"the_import_manifest")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=revision",
			absManifestPath,
		},
			"the_import_revision")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=remotebranch",
			absManifestPath,
		},
			"the_import_remotebranch")
	})
}
