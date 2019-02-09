// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"fuchsia.googlesource.com/jiri/project"
)

func TestPrefixTree(t *testing.T) {
	_, fakeroot, cleanup := setupUniverse(t)
	// only X.logger is needed
	defer cleanup()

	projects := []project.Project{
		project.Project{Name: "root", Path: "."},
		project.Project{Name: "a", Path: "a"},
		project.Project{Name: "b", Path: "b"},
		project.Project{Name: "c/d/e", Path: "c/d/e"},
		project.Project{Name: "c/d/f", Path: "c/d/f"},
		project.Project{Name: "c/d", Path: "c/d"},
		project.Project{Name: "c", Path: "c"},
	}
	expectedDropped := []project.Project{
		projects[0],
		projects[3],
		projects[4],
		projects[5],
	}

	// Fill projects into prefix tree
	root := prefixTree{nil, make(map[string]*prefixTree)}
	dropped := make(project.Projects)
	for _, v := range projects {
		addToTree(fakeroot.X, &root, v, dropped)
	}

	// Verify dropped nested projects
	failedDropped := func() {
		t.Logf("wrong nested projects list")
		t.Logf("expecting: ")
		for _, v := range expectedDropped {
			t.Logf("\tproject:%q", v.Path)
		}
		t.Logf("got:")
		for _, v := range dropped {
			t.Logf("\tproject:%q", v.Path)
		}
		t.Fail()
	}
	if len(dropped) != len(expectedDropped) {
		failedDropped()
	}

	for _, v := range expectedDropped {
		if _, ok := dropped[v.Key()]; !ok {
			failedDropped()
		}
	}

	// Verify the shape of prefix tree
	if len(root.next) == 3 {
		prefixes := []string{"a", "b", "c"}
		for _, v := range prefixes {
			if _, ok := root.next[v]; !ok {
				t.Errorf("root node does not contain project %q", v)
			}
		}
		for _, v := range root.next {
			if len(v.next) != 0 {
				t.Errorf("more than 1 level of nodes found in prefix tree")
			}
		}
	} else {
		t.Errorf("expecting %v first level nodes, but got %v", 3, len(root.next))
	}
}

func TestGitModules(t *testing.T) {
	_, fakeroot, cleanup := setupUniverse(t)
	defer cleanup()
	if err := fakeroot.UpdateUniverse(false); err != nil {
		t.Errorf("%v", err)
	}

	data, err := ioutil.ReadFile(fakeroot.X.JiriManifestFile())
	if err != nil {
		t.Errorf("%v", err)
	}

	localProjects, err := project.LocalProjects(fakeroot.X, project.FullScan)
	if err != nil {
		t.Errorf("scanning local fake project failed due to error %v", err)
	}

	localProjectsPathMap := make(map[string]project.Project)
	for _, v := range localProjects {
		v.Path, err = relativizePath(fakeroot.X.Root, v.Path)
		if err != nil {
			t.Errorf("path relativation failed due to error %v", err)
		}
		localProjectsPathMap[v.Path] = v
	}

	tempDir, err := ioutil.TempDir("", "gitmodules")
	if err != nil {
		t.Errorf(".gitmodules generation failed due to error %v", err)
	}
	defer os.RemoveAll(tempDir)
	genGitModuleFlags.genScript = path.Join(tempDir, "setup.sh")
	err = runGenGitModule(fakeroot.X, []string{path.Join(tempDir, ".gitmodules")})
	if err != nil {
		t.Errorf(".gitmodules generation failed due to error %v", err)
	}

	data, err = ioutil.ReadFile(genGitModuleFlags.genScript)
	if err != nil {
		t.Errorf("%v", err)
	}

	parsedScript, err := setupScriptParser(data)
	if err != nil {
		t.Errorf("failed to parse generated setup script due to error: %v", err)
	}

	if len(parsedScript) != len(localProjectsPathMap) {
		t.Errorf("expecting %v set up commands, got %v", len(localProjectsPathMap), len(parsedScript))
	}
	for k, v := range parsedScript {
		if p, ok := localProjectsPathMap[k]; ok {
			if p.Revision != v {
				t.Errorf("revision hash mismatch for project %q, expecting %q, got %q", k, p.Revision, v)
			}
		} else {
			t.Errorf("project at path %q does not match inital testing set up", k)
		}
	}

	data, err = ioutil.ReadFile(path.Join(tempDir, ".gitmodules"))
	if err != nil {
		t.Errorf("%v", err)
	}

	parsedModules, err := gitmoduleParser(data)
	if err != nil {
		t.Errorf("failed to parse generated .gitmodules due to error: %v", err)
	}

	if len(parsedModules) != len(localProjectsPathMap) {
		t.Errorf("expecting %v submodules, got %v", len(localProjectsPathMap), len(parsedModules))
	}
	for k, v := range parsedModules {
		if p, ok := localProjectsPathMap[k]; ok {
			if p.Name != v.Name || p.Remote != v.Remote || p.Revision != v.Revision || p.Path != v.Path {
				t.Errorf("submodule %q mismatch", p.Path)
			}
		} else {
			t.Errorf("project at path %q does not match inital testing set up", k)
		}
	}
}

func gitmoduleParser(data []byte) (map[string]project.Project, error) {
	var moduleBuf bytes.Buffer
	retMap := make(map[string]project.Project)
	moduleBuf.Write(data)
	moduleScanner := bufio.NewScanner(&moduleBuf)

	name := ""
	branch := ""
	subPath := ""
	remote := ""
	first := true
	for moduleScanner.Scan() {
		line := strings.TrimSpace(moduleScanner.Text())
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		if len(line) >= len("[submodue ") && strings.HasPrefix(line, "[submodule ") {
			if !first {
				if name == "" {
					return nil, fmt.Errorf("encountered a submodule with empty name")
				}
				if subPath == "" {
					return nil, fmt.Errorf("submodule %q has empty subPath", name)
				}
				// Unlike setup script, branch can be empty or non sha-1 hash in
				// .gitmodules file.
				retMap[subPath] = project.Project{Name: name, Path: subPath, Revision: branch, Remote: remote}
			}
			subPath = ""
			branch = ""
			remote = ""
			name = line[len("[submodule ") : len(line)-1]
			if name[0] != '"' || name[len(name)-1] != '"' {
				return nil, fmt.Errorf("submodule name %q is not quoted in generated gitmodules file", name)
			}
			name = name[1 : len(name)-1]
			first = false
			continue
		}

		fields := strings.Split(line, "=")
		if len(fields) != 2 {
			return nil, fmt.Errorf("unknown format while parsing %q", line)
		}
		fields[0] = strings.TrimSpace(fields[0])
		fields[1] = strings.TrimSpace(fields[1])
		switch fields[0] {
		case "url":
			remote = fields[1]
		case "path":
			subPath = fields[1]
		case "branch":
			branch = fields[1]
		default:
			return nil, fmt.Errorf("unknown attribute %q in %q", fields[0], line)
		}
	}
	if !first {
		if subPath == "" {
			return nil, fmt.Errorf("submodule %q has empty subPath", name)
		}
		retMap[subPath] = project.Project{Name: name, Path: subPath, Revision: branch, Remote: remote}
	}
	return retMap, nil
}

func setupScriptParser(data []byte) (map[string]string, error) {
	var scriptBuf bytes.Buffer
	retMap := make(map[string]string)
	scriptBuf.Write(data)
	scriptScanner := bufio.NewScanner(&scriptBuf)
	for scriptScanner.Scan() {
		line := scriptScanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		// git update-index --add --cacheinfo 160000 revision_hash path
		// In tests, the path will not contains white spaces
		fields := strings.Fields(line)
		if len(fields) != 7 || fields[0] != "git" ||
			fields[1] != "update-index" || fields[2] != "--add" ||
			fields[3] != "--cacheinfo" || fields[4] != "160000" {
			return nil, fmt.Errorf("illegal git command %q", line)
		}

		if _, err := hex.DecodeString(fields[5]); err != nil {
			return nil, fmt.Errorf("illegal revision hash in git command %q", line)
		}
		if fields[6][0] == '"' && fields[6][len(fields[6])-1] == '"' {
			fields[6] = fields[6][1 : len(fields[6])-1]
		}
		retMap[fields[6]] = fields[5]
	}
	return retMap, nil
}
