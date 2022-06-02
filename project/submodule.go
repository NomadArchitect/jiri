// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package project

import (
	"go.fuchsia.dev/jiri"
)

type SubmoduleState struct {
	Name                string
	Superproject        string
	SuperprojectEnabled bool
}

// getSuperprojectStates returns the superprojects that have submodules enabled.
func getSuperprojectStates(jirix *jiri.X, projects Projects) map[string]bool {
	superprojectStates := make(map[string]bool)
	for _, p := range projects {
		if p.GitSubmodules {
			superprojectStates[p.Name] = true
		}
	}
	return superprojectStates
}

// removeSubmodulesFromProjects removes verified submodules from jiri projects.
func removeSubmodulesFromProjects(jirix *jiri.X, projects Projects) Projects {
	var submoduleProjectKeys []ProjectKey
	superprojectStates := getSuperprojectStates(jirix, projects)
	for k, p := range projects {
		if superprojectStates[p.GitSubmoduleOf] {
			submoduleProjectKeys = append(submoduleProjectKeys, k)
		}
	}
	for _, k := range submoduleProjectKeys {
		delete(projects, k)
	}
	return projects
}
