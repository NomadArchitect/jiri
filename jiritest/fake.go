// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jiritest

import (
	"os"
	"path/filepath"
	"testing"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/gitutil"
	"go.fuchsia.dev/jiri/jiritest/xtest"
	"go.fuchsia.dev/jiri/project"
)

// FakeJiriRoot sets up a fake root under a tmp directory.
type FakeJiriRoot struct {
	X             *jiri.X
	Projects      map[string]string
	ProjectHashes map[string][]string
	remote        string
}

const (
	ManifestFileName    = "public"
	ManifestProjectPath = "manifest"
)
const (
	defaultDataDir      = "data"
	ManifestProjectName = "manifest"
)

// NewFakeJiriRoot returns a new FakeJiriRoot and a cleanup closure.  The
// closure must be run to cleanup temporary directories and restore the original
// environment; typically it is run as a defer function.
func NewFakeJiriRoot(t *testing.T) *FakeJiriRoot {
	// lockfiles are disabled in tests by defaults
	jirix := xtest.NewX(t)
	fake := &FakeJiriRoot{
		X:             jirix,
		Projects:      map[string]string{},
		ProjectHashes: make(map[string][]string),
	}

	// Create fake remote manifest projects.
	remoteDir := t.TempDir()
	fake.remote = remoteDir
	if err := fake.CreateRemoteProject(ManifestProjectPath); err != nil {
		t.Fatal(err)
	}
	// Create a fake manifest.
	manifestDir := filepath.Join(remoteDir, ManifestProjectPath)
	if err := os.MkdirAll(manifestDir, os.FileMode(0700)); err != nil {
		t.Fatal(err)
	}
	if err := fake.WriteRemoteManifest(&project.Manifest{}); err != nil {
		t.Fatal(err)
	}
	// Add the "manifest" project to the manifest.
	if err := fake.AddProject(project.Project{
		Name:   ManifestProjectName,
		Path:   ManifestProjectPath,
		Remote: fake.Projects[ManifestProjectName],
	}); err != nil {
		t.Fatal(err)
	}
	// Create a .jiri_manifest file which imports the manifest created above.
	if err := fake.WriteJiriManifest(&project.Manifest{
		Imports: []project.Import{
			{
				Manifest: ManifestFileName,
				Name:     ManifestProjectName,
				Remote:   filepath.Join(fake.remote, ManifestProjectPath),
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Update the contents of the fake instance based on  the information
	// recorded in the remote manifest.
	if err := fake.UpdateUniverse(false); err != nil {
		t.Fatal(err)
	}

	return fake
}

// AddProject adds the given project to a remote manifest.
func (fake FakeJiriRoot) AddProject(project project.Project) error {
	manifest, err := fake.ReadRemoteManifest()
	if err != nil {
		return err
	}
	manifest.Projects = append(manifest.Projects, project)
	return fake.WriteRemoteManifest(manifest)
}

// AddHook adds the given hook to a remote manifest.
func (fake FakeJiriRoot) AddHook(hook project.Hook) error {
	manifest, err := fake.ReadRemoteManifest()
	if err != nil {
		return err
	}
	manifest.Hooks = append(manifest.Hooks, hook)
	return fake.WriteRemoteManifest(manifest)
}

// AddImport adds the given import to a remote manifest.
func (fake FakeJiriRoot) AddImport(importProject project.Import) error {
	manifest, err := fake.ReadRemoteManifest()
	if err != nil {
		return err
	}
	manifest.Imports = append(manifest.Imports, importProject)
	return fake.WriteRemoteManifest(manifest)
}

// AddPackage adds the given package to a remote manifest.
func (fake FakeJiriRoot) AddPackage(pkg project.Package) error {
	manifest, err := fake.ReadRemoteManifest()
	if err != nil {
		return err
	}
	manifest.Packages = append(manifest.Packages, pkg)
	return fake.WriteRemoteManifest(manifest)
}

// DisableRemoteManifestPush disables pushes to the remote manifest
// repository.
func (fake FakeJiriRoot) DisableRemoteManifestPush() error {
	dir := gitutil.RootDirOpt(filepath.Join(fake.remote, ManifestProjectPath))
	return gitutil.New(fake.X, dir).Checkout("main")
}

// EnableRemoteManifestPush enables pushes to the remote manifest
// repository.
func (fake FakeJiriRoot) EnableRemoteManifestPush() error {
	dir := filepath.Join(fake.remote, ManifestProjectPath)
	scm := gitutil.New(fake.X, gitutil.RootDirOpt(dir))
	if ok, err := scm.BranchExists("non-main"); ok && err == nil {
		if err := scm.CreateBranch("non-main"); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return scm.Checkout("non-main")
}

// CreateRemoteProject creates a new remote project.
func (fake FakeJiriRoot) CreateRemoteProject(name string) error {
	projectDir := filepath.Join(fake.remote, name)
	if err := os.MkdirAll(projectDir, os.FileMode(0700)); err != nil {
		return err
	}
	if err := gitutil.New(fake.X).Init(projectDir); err != nil {
		return err
	}
	git := gitutil.New(fake.X, gitutil.RootDirOpt(projectDir))
	if err := git.Config("user.email", "john.doe@example.com"); err != nil {
		return err
	}
	if err := git.Config("user.name", "John Doe"); err != nil {
		return err
	}

	if err := git.CommitWithMessage("initial commit"); err != nil {
		return err
	}

	hash, err := git.CurrentRevisionOfBranch("HEAD")
	if err != nil {
		return err
	}
	fake.ProjectHashes[name] = append(fake.ProjectHashes[name], hash)
	fake.Projects[name] = projectDir
	return nil
}

// ReadRemoteManifest read a manifest from the remote manifest project.
func (fake FakeJiriRoot) ReadRemoteManifest() (*project.Manifest, error) {
	path := filepath.Join(fake.remote, ManifestProjectPath, ManifestFileName)
	return project.ManifestFromFile(fake.X, path)
}

// UpdateUniverse synchronizes the content of the Vanadium fake based
// on the content of the remote manifest.
func (fake FakeJiriRoot) UpdateUniverse(gc bool) error {
	return project.UpdateUniverse(fake.X, project.UpdateUniverseParams{
		GC:                   gc,
		RunHooks:             true,
		FetchPackages:        true,
		RunHookTimeout:       project.DefaultHookTimeout,
		FetchPackagesTimeout: project.DefaultPackageTimeout,
	})
}

// ReadJiriManifest reads the .jiri_manifest manifest.
func (fake FakeJiriRoot) ReadJiriManifest() (*project.Manifest, error) {
	return project.ManifestFromFile(fake.X, fake.X.JiriManifestFile())
}

// WriteJiriManifest writes the given manifest to the .jiri_manifest file.
func (fake FakeJiriRoot) WriteJiriManifest(manifest *project.Manifest) error {
	return manifest.ToFile(fake.X, fake.X.JiriManifestFile())
}

// WriteRemoteManifest writes the given manifest to the remote
// manifest project.
func (fake FakeJiriRoot) WriteRemoteManifest(manifest *project.Manifest) error {
	dir := filepath.Join(fake.remote, ManifestProjectPath)
	path := filepath.Join(dir, ManifestFileName)
	return fake.writeManifest(manifest, dir, path)
}

func (fake FakeJiriRoot) writeManifest(manifest *project.Manifest, dir, path string) error {
	git := gitutil.New(fake.X, gitutil.UserNameOpt("John Doe"), gitutil.UserEmailOpt("john.doe@example.com"), gitutil.RootDirOpt(dir))
	if err := manifest.ToFile(fake.X, path); err != nil {
		return err
	}
	if err := git.Add(path); err != nil {
		return err
	}
	if err := git.Commit(); err != nil {
		return err
	}
	hash, err := git.CurrentRevisionOfBranch("HEAD")
	if err != nil {
		return err
	}
	fake.ProjectHashes[ManifestProjectName] = append(fake.ProjectHashes[ManifestProjectName], hash)
	return nil
}

// AddProjectOverride adds a project override into .jiri_manifest of current FakeJiriRoot.
func (fake FakeJiriRoot) AddProjectOverride(name, remote, revision string) error {
	m, err := fake.ReadJiriManifest()
	if err != nil {
		return err
	}
	m.ProjectOverrides = append(m.ProjectOverrides, project.Project{
		Name:     name,
		Remote:   remote,
		Revision: revision,
	})
	fake.WriteJiriManifest(m)
	return nil
}

// AddImportOverride adds a import override into .jiri_manifest of current FakeJiriRoot.
func (fake FakeJiriRoot) AddImportOverride(name, remote, revision, manifest string) error {
	m, err := fake.ReadJiriManifest()
	if err != nil {
		return err
	}
	m.ImportOverrides = append(m.ImportOverrides, project.Import{
		Name:     name,
		Remote:   remote,
		Revision: revision,
		Manifest: manifest,
	})
	fake.WriteJiriManifest(m)
	return nil
}
