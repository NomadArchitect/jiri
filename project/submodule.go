// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package project

import (
	"regexp"
	"strings"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/gitutil"
)

type Submodule struct {
	Name string
	Path string
	// Submodule SHA-1 prefix, could be "+", "-" or "U".
	// "-" means submodule not initialized.
	Prefix       string
	Remote       string
	Revision     string
	Superproject string
}

type Submodules map[string]Submodule

// containSubmodules checks if any of the projects contain submodules.
func containSubmodules(jirix *jiri.X, projects Projects) bool {
	for _, p := range projects {
		if p.GitSubmodules {
			if isSuperproject := isSuperproject(jirix, p); isSuperproject {
				return true
			}
		}
	}
	return false
}

// getAllSubmodules return all submodules states
func getAllSubmodules(jirix *jiri.X, projects Projects) []Submodules {
	var allSubmodules []Submodules
	for _, p := range projects {
		if p.GitSubmodules {
			if submodules := getSubmoduleStatus(jirix, p); submodules != nil {
				allSubmodules = append(allSubmodules, submodules)
			}
		}
	}
	return allSubmodules
}

// getSubmoduleStatus returns submodule states in superproject.
func getSubmoduleStatus(jirix *jiri.X, superproject Project) Submodules {
	scm := gitutil.New(jirix, gitutil.RootDirOpt(superproject.Path))
	submoduleStatus, _ := scm.SubmoduleStatus()
	submodulesConfig := strings.Split(string(submoduleStatus), "\n")
	submodules := make(Submodules)
	for _, subm := range submodulesConfig {
		var submodule Submodule
		re := regexp.MustCompile(`[-+U]?([a-fA-F0-9]{40})\s(.*?)\s`)
		pre := regexp.MustCompile(`([-+U])?`)
		submConfig := re.FindAllString(subm, 1)[0]
		submodule.Prefix = pre.FindAllString(subm, 1)[0]
		submodule.Revision = strings.Split(submConfig, " ")[0]
		submodule.Name = strings.Split(submConfig, " ")[1]
		submodule.Remote, _ = scm.SubmoduleUrl(submodule.Name)
		submodule.Superproject = superproject.Name
		submodules[submodule.Name] = submodule
		if submodule.Prefix == "+" {
			jirix.Logger.Warningf("Submodule %s current checkout does not match the SHA-1 to the index of the containing repository.", submodule.Name)
		}
		if submodule.Prefix == "U" {
			jirix.Logger.Warningf("Submodule %s has merge conflicts.", submodule.Name)
		}
	}
	return submodules
}

// getSuperprojectStates returns the superprojects that have submodules enabled.
func getSuperprojectStates(projects Projects) map[string]bool {
	superprojectStates := make(map[string]bool)
	for _, p := range projects {
		if p.GitSubmodules {
			superprojectStates[p.Name] = true
		}
	}
	return superprojectStates
}

// isSuperproject checks if submodules exist under a project
func isSuperproject(jirix *jiri.X, project Project) bool {
	submodules := getSubmoduleStatus(jirix, project)
	for _, subm := range submodules {
		if subm.Prefix != "-" {
			return true
		}
	}
	return false
}

// removeSubmodulesFromProjects removes verified submodules from jiri projects.
func removeSubmodulesFromProjects(projects Projects) Projects {
	var submoduleProjectKeys []ProjectKey
	superprojectStates := getSuperprojectStates(projects)
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
