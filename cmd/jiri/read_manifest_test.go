package main

import (
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
	readManifestCallback = func(_ *jiri.X, _ string) (*project.Manifest, error) {
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

	// Expects runReadManifest to return a specific value when given args.
	expectAttributeValue := func(t *testing.T, args []string, expectedValue string) {
		// Set up a fake Jiri root to pass to our command.
		fake, cleanup := jiritest.NewFakeJiriRoot(t)
		defer cleanup()

		// Parse command line flags
		cmdReadManifest.Flags.Parse(args)

		// Jiri does a strange rotating-of-the-flags and places the positional
		// command-line arguments before the --flag arguments. Perform the same
		// rotation before passing args to the subcommand.
		argsLen := len(args)
		args = append(args[argsLen-1:], args[0:argsLen-1]...)

		// Run the command.
		var stderr string
		runCmd := func() {
			if err := runReadManifest(fake.X, args); err != nil {
				// Capture the error as stderr since Jiri subcommands don't
				// intenionally print to stderr when they fail.
				stderr = err.Error()
			}
		}
		stdout, _, err := runfunc(runCmd)
		if err != nil {
			t.Fatal(err)
		}

		// If an error occurred, fail.
		if stderr != "" {
			t.Error("error:", stderr)
			return
		}

		// Compare stdout to the expected value
		if stdout != expectedValue {
			t.Errorf("expected %s, got %s", expectedValue, stdout)
		}
	}

	// Expects runReadManifest to error when given args.
	expectError := func(t *testing.T, args []string) {
		// Set up a fake Jiri root to pass to our command.
		fake, cleanup := jiritest.NewFakeJiriRoot(t)
		defer cleanup()

		// Parse command line flags
		cmdReadManifest.Flags.Parse(args)

		// Jiri does a strange rotating-of-the-flags and places the positional
		// command-line arguments before the --flag arguments. Perform the same
		// rotation before passing args to the subcommand.
		argsLen := len(args)
		args = append(args[argsLen-1:], args[0:argsLen-1]...)

		// Run the command.
		var stderr string
		runCmd := func() {
			if err := runReadManifest(fake.X, args); err != nil {
				// Capture the error as stderr since Jiri subcommands don't
				// intenionally print to stderr when they fail.
				stderr = err.Error()
			}
		}
		stdout, _, err := runfunc(runCmd)
		if err != nil {
			t.Fatal(err)
		}

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
