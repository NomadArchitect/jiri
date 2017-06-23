// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/log"
	"fuchsia.googlesource.com/jiri/project"
)

var diffFlags struct {
	cls          bool
	indentOutput bool

	// Need this to avoid infinite loop
	maxCls uint
}

var cmdDiff = &cmdline.Command{
	Runner:   jiri.RunnerFunc(runDiff),
	Name:     "diff",
	Short:    "Prints diff between two snapshots",
	ArgsName: "<snapshot-1> <snapshot-2>",
	ArgsLong: "<snapshot-1/2> are files or urls containing snapshot",
	Long: `
Prints diff between two snapshots in json format. Max CLs returned for a
project is controlled by flag max-xls and is default by 5. The format of
returned json:
{
	new_projects: [
		{
			name: name,
			path: path,
			remote: remote,
			revision: rev
		},{...}...
	],
	deleted_projects:[
		{
			name: name,
			path: path,
			remote: remote,
			revision: rev
		},{...}...
	],
	updated_projects:[
		{
			name: name,
			path: path,
			remote: remote,
			revision: rev
			old_revision: old-rev, // if updated
			old_path: old-path //if moved
			cls:[
				{
					number: num,
					url: url,
					commit: commit,
					subject:sub
					changeId:changeId
				},{...},...
			]
			has_more_cls: true, 
			error: error in retrieving CL
		},{...}...
	]
}
`,
}

func init() {
	flags := &cmdDiff.Flags
	flags.BoolVar(&diffFlags.cls, "cls", true, "Return CLs for changed projects.")
	flags.BoolVar(&diffFlags.indentOutput, "indent", true, "Indent json output.")
	flags.UintVar(&diffFlags.maxCls, "max-cls", 5, "Max number of CLs returned per changed project.")
}

type DiffCl struct {
	Commit   string `json:"commit"`
	Number   string `json:"number"`
	Url      string `json:"url"`
	Subject  string `json:"subject"`
	ChangeId string `json:"changeId"`
}

type DiffProject struct {
	Name        string   `json:"name"`
	Remote      string   `json:"remote"`
	Path        string   `json:"path"`
	OldPath     string   `json:"old_path,omitempty"`
	Revision    string   `json:"revision"`
	OldRevision string   `json:"old_revision,omitempty"`
	Cls         []DiffCl `json:"cls,omitempty"`
	Error       string   `json:"error,omitempty"`
	HasMoreCls  bool     `json:"has_more_cls,omitempty"`
}

type DiffProjectsByName []DiffProject

func (p DiffProjectsByName) Len() int {
	return len(p)
}
func (p DiffProjectsByName) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p DiffProjectsByName) Less(i, j int) bool {
	return p[i].Name < p[j].Name
}

type Diff struct {
	NewProjects     []DiffProject `json:"new_projects"`
	DeletedProjects []DiffProject `json:"deleted_projects"`
	UpdatedProjects []DiffProject `json:"updated_projects"`
}

func (d *Diff) Sort() *Diff {
	sort.Sort(DiffProjectsByName(d.NewProjects))
	sort.Sort(DiffProjectsByName(d.DeletedProjects))
	sort.Sort(DiffProjectsByName(d.UpdatedProjects))
	return d
}

func runDiff(jirix *jiri.X, args []string) error {
	if len(args) != 2 {
		return jirix.UsageErrorf("Please provide two snapshots to diff")
	}
	d, err := getDiff(jirix, args[0], args[1])
	if err != nil {
		return err
	}
	var bytes []byte
	if diffFlags.indentOutput {
		bytes, err = json.MarshalIndent(d, "", " ")
	} else {
		bytes, err = json.Marshal(d)
	}
	if err != nil {
		return err
	} else {
		fmt.Println(string(bytes))
	}
	return nil
}

func getDiff(jirix *jiri.X, snapshot1, snapshot2 string) (*Diff, error) {
	diff := &Diff{
		NewProjects:     make([]DiffProject, 0),
		DeletedProjects: make([]DiffProject, 0),
		UpdatedProjects: make([]DiffProject, 0),
	}
	oldLogger := jirix.Logger
	defer func() {
		jirix.Logger = oldLogger
	}()
	jirix.Logger = log.NewLogger(log.NoLogLevel, jirix.Color)
	projects1, _, err := project.LoadSnapshotFile(jirix, snapshot1)
	if err != nil {
		return nil, err
	}
	projects2, _, err := project.LoadSnapshotFile(jirix, snapshot2)
	if err != nil {
		return nil, err
	}
	project.MatchLocalWithRemote(projects1, projects2)
	jirix.Logger = oldLogger

	// Get deleted projects
	for key, p1 := range projects1 {
		if _, ok := projects2[key]; !ok {
			diff.DeletedProjects = append(diff.DeletedProjects, DiffProject{
				Name:     p1.Name,
				Remote:   p1.Remote,
				Path:     p1.Path,
				Revision: p1.Revision,
			})
		}
	}

	// Get new projects and also extract updated projects
	var updatedProjectKeys project.ProjectKeys
	for key, p2 := range projects2 {
		if p1, ok := projects1[key]; !ok {
			diff.NewProjects = append(diff.NewProjects, DiffProject{
				Name:     p2.Name,
				Remote:   p2.Remote,
				Path:     p2.Path,
				Revision: p2.Revision,
			})
		} else {
			if p1.Path != p2.Path || p1.Revision != p2.Revision {
				updatedProjectKeys = append(updatedProjectKeys, key)
			}
		}
	}

	for _, key := range updatedProjectKeys {
		p1 := projects1[key]
		p2 := projects2[key]
		diffP := DiffProject{
			Name:     p2.Name,
			Remote:   p2.Remote,
			Path:     p2.Path,
			Revision: p2.Revision,
		}
		if p1.Path != p2.Path {
			diffP.OldPath = p1.Path
		}
		if p1.Revision != p2.Revision {
			diffP.OldRevision = p1.Revision
		}
		diff.UpdatedProjects = append(diff.UpdatedProjects, diffP)
	}
	return diff.Sort(), nil
}
