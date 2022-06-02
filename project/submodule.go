// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package project

import (
	"fmt"

	"go.fuchsia.dev/jiri"
)

type SubmoduleState struct {
	Name                string
	Superproject        string
	SuperprojectEnabled bool
}

func getSubmoduleStates(jirix *jiri.X, projects Projects) (map[string]bool, map[string]SubmoduleState) {
	var submoduleStates = map[string]SubmoduleState{}
	// Keep track of which superproject has submodules turned on. E.g. "fuchsia".
	superprojects := getSuperprojectStates(jirix, projects)
	for _, p := range projects {
		if p.GitSubmoduleOf != "" {
			fmt.Printf("YupingDebugger: setting submodulestate for %+v \n", p)
			if superprojects[p.GitSubmoduleOf] {
				var submState SubmoduleState
				submState.Name = p.Name
				submState.Superproject = p.GitSubmoduleOf
				submState.SuperprojectEnabled = superprojects[p.GitSubmoduleOf]
				submoduleStates[p.Name] = submState
				fmt.Printf("YupingDebugger: submodule state: %+v\n", submState)
			}
		}
		fmt.Printf("YupingDebugger: current project %+v\n", p.Name)
	}
	fmt.Printf("YupingDebugger: superproject states %+v\n", superprojects)
	fmt.Printf("YupingDebugger: projects in submodulestate func %+v\n", projects)
	return superprojects, submoduleStates
}

func getSuperprojectStates(jirix *jiri.X, projects Projects) map[string]bool {
	superprojectStates := make(map[string]bool)
	for _, p := range projects {
		if p.GitSubmodules {
			superprojectStates[p.Name] = true
			fmt.Printf("YupingDebugger: superproject labeled: %+v\n", p)
		}
	}
	return superprojectStates
}
