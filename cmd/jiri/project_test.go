// Copyright 2020 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"fuchsia.googlesource.com/jiri/jiritest"
	"fuchsia.googlesource.com/jiri/project"
)

func TestProjectIgnoresByAttribute(t *testing.T) {
	fake, cleanup := jiritest.NewFakeJiriRoot(t)
	defer cleanup()

	// Set up projects and packages with explict attributes
	numProjects := 3
	numOptionalProjects := 2
	localProjects := []project.Project{}
	totalProjects := numProjects + numOptionalProjects

	createRemoteProj := func(i int, attributes string) {
		name := projectName(i)
		path := fmt.Sprintf("path-%d", i)
		if err := fake.CreateRemoteProject(name); err != nil {
			t.Fatalf("failed to create remote project due to error: %v", err)
		}
		p := project.Project{
			Name:       name,
			Path:       filepath.Join(fake.X.Root, path),
			Remote:     fake.Projects[name],
			Attributes: attributes,
		}
		localProjects = append(localProjects, p)
		if err := fake.AddProject(p); err != nil {
			t.Fatalf("failed to add a project to manifest due to error: %v", err)
		}
	}

	for i := 0; i < numProjects; i++ {
		createRemoteProj(i, "")
	}

	for i := numProjects; i < totalProjects; i++ {
		createRemoteProj(i, "optional")
	}

	// Create initial commit in each repo.
	for _, remoteProjectDir := range fake.Projects {
		writeReadme(t, fake.X, remoteProjectDir, "initial readme")
	}

	// Try default mode
	fake.X.FetchingAttrs = ""
	fake.UpdateUniverse(true)

	file, err := ioutil.TempFile("", "tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	jsonOutputFlag = file.Name()
	useRemoteProjects = true

	runProject(fake.X, []string{})

	var projectInfo []projectInfoOutput

	fileinfo, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	filesize := fileinfo.Size()
	buffer := make([]byte, filesize)

	_, err = file.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(buffer, &projectInfo)
	if err != nil {
		t.Fatal(err)
	}

	if len(projectInfo) != numProjects {
		t.Errorf("Unexpected number of projects returned (%d, %d) (want, got)\n%v", numProjects, len(projectInfo), projectInfo)
	}

	// Try attributes
	fake.X.FetchingAttrs = "optional"
	fake.UpdateUniverse(true)

	file2, err := ioutil.TempFile("", "tmp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file2.Name())

	jsonOutputFlag = file2.Name()
	useRemoteProjects = true

	runProject(fake.X, []string{})

	fileinfo, err = file2.Stat()
	if err != nil {
		t.Fatal(err)
	}

	filesize = fileinfo.Size()
	buffer = make([]byte, filesize)

	_, err = file.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(buffer, &projectInfo)
	if err != nil {
		t.Fatal(err)
	}

	if len(projectInfo) != totalProjects {
		t.Errorf("Unexpected number of projects returned (%d, %d) (want, got)\n%v", totalProjects, len(projectInfo), projectInfo)
	}
}