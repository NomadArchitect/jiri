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

// getSubmoduleStates finds states of the projects that are submodules of a superproject, and
// make sure only superprojects with submodules enabled are included.
func getSubmoduleStates(jirix *jiri.X, projects Projects) map[string]SubmoduleState {
	var submoduleStates = map[string]SubmoduleState{}
	// Keep track of which superproject has submodules turned on. E.g. "fuchsia".
	superprojects := getSuperprojectStates(jirix, projects)
	for _, p := range projects {
		if p.GitSubmoduleOf != "" {
			if superprojects[p.GitSubmoduleOf] {
				var submState SubmoduleState
				submState.Name = p.Name
				submState.Superproject = p.GitSubmoduleOf
				submState.SuperprojectEnabled = superprojects[p.GitSubmoduleOf]
				submoduleStates[p.Name] = submState
			}
		}
	}
	return submoduleStates
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
func removeSubmodulesFromProjects(jirix *jiri.X, submoduleStates map[string]SubmoduleState, projects Projects) Projects {
	removedProjects := make(map[string]ProjectKey)
	for _, p := range projects {
		if s, ok := submoduleStates[p.Name]; ok {
			if s.SuperprojectEnabled {
				removedProjects[p.Name] = p.Key()
			}
		}
	}
	for _, p_key := range removedProjects {
		delete(projects, p_key)
	}
	return projects
}
