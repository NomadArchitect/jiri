package main

import (
	"flag"
	"testing"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/jiritest"
	"fuchsia.googlesource.com/jiri/project"
)

func TestReadManifest(t *testing.T) {
	// Override the callback to read a manifest file with our own. (This
	// callback is declared in project.go). Overriding this allows us to avoid
	// placing a manifest in the file-tree to run this test, and prevents
	// changes to such a manifest from breaking these tests.
	testReadManifest := func(_ *jiri.X, _ string) (*project.Manifest, error) {
		return project.ManifestFromBytes([]byte(`
<?xml version="1.0" encoding="UTF-8"?>
<manifest>
  <imports>
	<import name="the_import"
			manifest="the_import_manifest"
			remote="https://fuchsia.googlesource.com/the_import"
			revision="the_import_revision"
			remotebranch="the_import_remotebranch"
			root="the_import_root"/>
  </imports>
  <projects>
	<project name="the_project"
			 path="path/to/the_project"
			 remote="https://fuchsia.googlesource.com/the_project"
			 remotebranch="the_project_remotebranch"
			 revision="the_project_revision"
			 githooks="the_project_githooks"
			 gerrithost="https://fuchsia-review.googlesource.com"
			 historydepth="2"/>
 </projects>
</manifest>
`))
	}

	runCommand := func(t *testing.T, args []string) (stdout string, stderr string) {
		// Set up a fake Jiri root to pass to our command.
		fake, cleanup := jiritest.NewFakeJiriRoot(t)
		defer cleanup()

		// Create a new ReadManifestCommand with our test callback.
		cmd := &ReadManifestCommand{readManifestCallback: testReadManifest}

		// Initialize flags for the command.
		flagSet := flag.NewFlagSet("read-manifest-test", flag.ContinueOnError)
		cmd.SetFlags(flagSet)
		flagSet.Parse(args)

		// Run the command.
		runCmd := func() {
			if err := cmd.Run(fake.X, flagSet.Args()); err != nil {
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
			"path/to/manifest",
		})

		expectError(t, []string{
			"-element=the_project",
			"path/to/manifest",
		})
	})

	t.Run("should fail if -element is missing", func(t *testing.T) {
		expectError(t, []string{
			"-attribute=name",
			"path/to/manifest",
		})

		expectError(t, []string{
			"-attribute=name",
			"path/to/manifest",
		})
	})

	t.Run("should read <project> attributes", func(t *testing.T) {
		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=name",
			"path/to/manifest",
		},
			"the_project")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=remote",
			"the_project_remote",
		},
			"https://fuchsia.googlesource.com/the_project")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=revision",
			"path/to/manifest",
		},
			"the_project_revision")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=remotebranch",
			"path/to/manifest",
		},
			"the_project_remotebranch")

		expectAttributeValue(t, []string{
			"-element=the_project",
			"-attribute=path",
			"path/to/manifest",
		},
			"path/to/the_project")
	})

	t.Run("should read <import> attributes", func(t *testing.T) {
		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=name",
			"path/to/manifest",
		},
			"the_import")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=remote",
			"path/to/manifest",
		},
			"https://fuchsia.googlesource.com/the_import")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=manifest",
			"path/to/manifest",
		},
			"the_import_manifest")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=revision",
			"path/to/manifest",
		},
			"the_import_revision")

		expectAttributeValue(t, []string{
			"-element=the_import",
			"-attribute=remotebranch",
			"path/to/manifest",
		},
			"the_import_remotebranch")
	})
}
