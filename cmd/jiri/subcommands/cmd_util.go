// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package subcommands

import (
	"fmt"
	"path"
	"path/filepath"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/project"
)

// currentProject returns the Project containing the current working directory.
// The current working directory must be inside root.
func currentProject(jirix *jiri.X) (project.Project, error) {
	dir := jirix.Cwd

	// Walk up the path until we find a project at that path, or hit the jirix.Root parent.
	// Note that we can't just compare path prefixes because of soft links.
	for dir != path.Dir(jirix.Root) && dir != string(filepath.Separator) {
		if isLocal, err := project.IsLocalProject(jirix, dir); err != nil {
			return project.Project{}, fmt.Errorf("Error while checking for local project at path %q: %s", dir, err)
		} else if !isLocal {
			dir = filepath.Dir(dir)
			continue
		}
		p, err := project.ProjectAtPath(jirix, dir)
		if err != nil {
			return project.Project{}, fmt.Errorf("Error while getting project at path %q: %s", dir, err)
		}
		return p, nil
	}
	return project.Project{}, fmt.Errorf("directory %q is not contained in a project", dir)
}

// getDefaultLocalManifestProjects essentially converts the boolean `-local-manifest=true`
// flag to the repeated `-local-manifest-project` flag. The default is to only include
// the root manifest project.
func getDefaultLocalManifestProjects(jirix *jiri.X) ([]string, error) {
	manifest, err := project.ManifestFromFile(jirix, jirix.JiriManifestFile())
	if err != nil {
		return nil, err
	}
	var localManifestProjects []string
	for _, imp := range manifest.Imports {
		localManifestProjects = append(localManifestProjects, imp.Name)
	}
	return localManifestProjects, nil
}
