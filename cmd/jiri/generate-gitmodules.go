// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/project"
)

var cmdGenGitModule = &cmdline.Command{
	Runner: jiri.RunnerFunc(runGenGitModule),
	Name:   "generate-gitmodules",
	Short:  "Create a .gitmodule file for git submodule repository",
	Long: `
The "jiri generate-gitmodules <.gitmodule path>" command captures the current project state
and create a .gitmodules file.
`,
	ArgsName: "<.gitmodule path>",
	ArgsLong: "<.gitmodule path> is the path to the output .gitmodule file.",
}

var genGitModuleFlags struct {
	genScript    string
	redirectRoot bool
}

func init() {
	flags := &cmdGenGitModule.Flags
	flags.StringVar(&genGitModuleFlags.genScript, "generate-script", "", "File to save generated git commands for seting up a superproject.")
	flags.BoolVar(&genGitModuleFlags.redirectRoot, "redir-root", false, "When set to true, jiri will add the root repository as a submodule into {name}-mirror directory and create necessary setup commands in generated script.")
}

type prefixTree struct {
	project *project.Project
	next    map[string]*prefixTree
}

func runGenGitModule(jirix *jiri.X, args []string) error {
	var gitmodulesPath = ".gitmodules"
	if len(args) == 1 {
		gitmodulesPath = args[0]
	}
	if len(args) > 1 {
		return jirix.UsageErrorf("unexpected number of arguments")
	}

	localProjects, err := project.LocalProjects(jirix, project.FullScan)
	if err != nil {
		return err
	}
	return writeGitModules(jirix, localProjects, gitmodulesPath)
}

func addToTree(jirix *jiri.X, root *prefixTree, proj project.Project, dropped project.Projects) error {
	if root == nil {
		return errors.New("addToTree called with nil root pointer")
	}

	if proj.Path == "." || proj.Path == "" || proj.Path == string(filepath.Separator) {
		// Skip fuchsia.git project
		dropped[proj.Key()] = proj
		return nil
	}

	// Walk the prefix tree to look for nested projects
	elmts := strings.Split(proj.Path, string(filepath.Separator))
	pin := root
	for i := 0; i < len(elmts); i++ {
		if next, ok := pin.next[elmts[i]]; ok {
			if next.project != nil {
				// proj is nested under next.project, drop proj
				jirix.Logger.Debugf("project %q:%q nested under project %q:%q", proj.Path, proj.Remote, proj.Path, next.project.Remote)
				dropped[proj.Key()] = proj
				return nil
			}
			pin = next
		} else {
			next = &prefixTree{nil, make(map[string]*prefixTree)}
			pin.next[elmts[i]] = next
			pin = next
		}
	}
	if len(pin.next) != 0 {
		// There is one or more project nested under proj.
		jirix.Logger.Debugf("following project nested under project %q:%q", proj.Path, proj.Remote)
		purgeLeaves(jirix, pin, dropped)
		jirix.Logger.Debugf("\n")
	}
	pin.project = &proj
	return nil
}

func purgeLeaves(jirix *jiri.X, node *prefixTree, dropped project.Projects) error {
	// Looking for projects nested under node using BFS
	workList := make([]*prefixTree, 0)
	workList = append(workList, node)

	for len(workList) > 0 {
		item := workList[0]
		if item == nil {
			return errors.New("purgeLeaves encountered a nil node")
		}
		workList = workList[1:]
		if item.project != nil {
			dropped[item.project.Key()] = *item.project
			jirix.Logger.Debugf("\tnested project %q:%q", item.project.Path, item.project.Remote)
		}
		for _, v := range item.next {
			workList = append(workList, v)
		}
	}

	// Purge leaves under node
	node.next = make(map[string]*prefixTree)
	return nil
}

func writeGitModules(jirix *jiri.X, projects project.Projects, outputPath string) error {
	projEntries := make([]project.Project, len(projects))

	// relativaize the paths and copy projects from map to slice for sorting.
	i := 0
	for _, v := range projects {
		relPath, err := relativizePath(jirix.Root, v.Path)
		if err != nil {
			return err
		}
		v.Path = relPath
		projEntries[i] = v
		i++
	}
	sort.Slice(projEntries, func(i, j int) bool {
		return string(projEntries[i].Key()) < string(projEntries[j].Key())
	})

	// Create path prefix tree to collect all nested projects
	root := prefixTree{nil, make(map[string]*prefixTree)}
	dropped := make(project.Projects)
	for _, v := range projEntries {
		addToTree(jirix, &root, v, dropped)
	}

	// Start creating .gitmodule and set up script.
	var gitmoduleBuf bytes.Buffer
	var commandBuf bytes.Buffer
	commandBuf.WriteString("#/!bin/sh\n")

	// Special hack for fuchsia.git
	// When -redir-root is set to true, fuchsia.git will be added as submodule
	// to fuchsia-mirror directory
	reRootRepoName := ""
	if genGitModuleFlags.redirectRoot {
		// looking for root repository, there should be no more than 1
		rIndex := -1
		for i, v := range projEntries {
			if v.Path == "." || v.Path == "" || v.Path == string(filepath.Separator) {
				if rIndex == -1 {
					rIndex = i
				} else {
					return fmt.Errorf("more than 1 project defined at path \".\", projects %+v:%+v", projEntries[rIndex], projEntries[i])
				}
			}
		}
		if rIndex != -1 {
			v := projEntries[rIndex]
			v.Name = v.Name + "-mirror"
			v.Path = v.Name
			gitmoduleBuf.WriteString(moduleDecl(v))
			gitmoduleBuf.WriteString("\n")
			commandBuf.WriteString(commandDecl(v))
			commandBuf.WriteString("\n")
		}
	}

	for _, v := range projEntries {
		if reRootRepoName != "" && reRootRepoName == v.Path {
			return fmt.Errorf("path collision for root repo and project %+v", v)
		}
		if _, ok := dropped[v.Key()]; ok {
			jirix.Logger.Debugf("dropped project %v+", v)
			continue
		}
		gitmoduleBuf.WriteString(moduleDecl(v))
		gitmoduleBuf.WriteString("\n")
		commandBuf.WriteString(commandDecl(v))
		commandBuf.WriteString("\n")
	}
	jirix.Logger.Debugf("generated gitmodule content \n%v\n", gitmoduleBuf.String())
	if err := ioutil.WriteFile(outputPath, gitmoduleBuf.Bytes(), 0644); err != nil {
		return err
	}

	if genGitModuleFlags.genScript != "" {
		jirix.Logger.Debugf("generated set up script for gitmodule content \n%v\n", commandBuf.String())
		if err := ioutil.WriteFile(genGitModuleFlags.genScript, commandBuf.Bytes(), 0755); err != nil {
			return err
		}
	}
	return nil
}

func relativizePath(basepath, targpath string) (string, error) {
	if filepath.IsAbs(targpath) {
		relPath, err := filepath.Rel(basepath, targpath)
		if err != nil {
			return "", err
		}
		return relPath, nil
	}
	return targpath, nil
}

func moduleDecl(p project.Project) string {
	tmpl := "[submodule \"%s\"]\n\tbranch = %s\n\tpath = %s\n\turl = %s"
	return fmt.Sprintf(tmpl, p.Name, p.Revision, p.Path, p.Remote)
}

func commandDecl(p project.Project) string {
	tmpl := "git update-index --add --cacheinfo 160000 %s \"%s\""
	return fmt.Sprintf(tmpl, p.Revision, p.Path)
}
