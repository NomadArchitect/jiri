// Copyright 2019 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"text/template"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cipd"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/project"
)

// cmd represents the "jiri project" command.
var cmdPackage = &cmdline.Command{
	Runner: jiri.RunnerFunc(runPackageInfo),
	Name:   "package",
	Short:  "Display the jiri packages",
	Long: `Display structured info on the existing
	packages and branches. Packages are specified using either names or	regular
	expressions that are matched against package names. If no command line
	arguments are provided all projects will be used.`,
	ArgsName: "<package ...>",
	ArgsLong: "<package ...> is a list of packages to give info about.",
}

// packageInfoOutput defines JSON format for 'project info' output.
type packageInfoOutput struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Manifest string `json:"manifest,omitempty"`
}

func init() {
	cmdPackage.Flags.StringVar(&jsonOutputFlag, "json-output", "", "Path to write operation results to.")
	cmdPackage.Flags.BoolVar(&regexpFlag, "regexp", false, "Use argument as regular expression.")
}

// runPackageInfo provides structured info on packages.
func runPackageInfo(jirix *jiri.X, args []string) error {
	var err error

	regexps := []*regexp.Regexp{}
	if len(args) > 0 && regexpFlag {
		regexps = make([]*regexp.Regexp, len(args), len(args))
		for i, a := range args {
			re, err := regexp.Compile(a)
			if err != nil {
				return fmt.Errorf("failed to compile regexp %v: %v", a, err)
			}
			regexps[i] = re
		}
	}

	projects, err := project.LocalProjects(jirix, project.FastScan)
	if err != nil {
		return err
	}
	_, _, pkgs, err := project.LoadManifestFile(jirix, jirix.JiriManifestFile(), projects, true)
	if err != nil {
		return err
	}
	var keys project.PackageKeys
	if len(args) != 0 {
		for k, v := range pkgs {
			if regexpFlag {
				for _, re := range regexps {
					if re.MatchString(v.Name) {
						keys = append(keys, k)
						break
					}
				}
			} else {
				for _, arg := range args {
					if arg == v.Name {
						keys = append(keys, k)
						break
					}
				}
			}
		}
	} else {
		for k := range pkgs {
			keys = append(keys, k)
		}
	}

	sort.Sort(keys)

	info := make([]packageInfoOutput, len(keys))
	for i, key := range keys {
		pkg := pkgs[key]
		pkgPath, err := pkg.GetPath()
		if err != nil {
			return err
		}
		tmpl, err := template.New("pack").Parse(pkgPath)
		if err != nil {
			return fmt.Errorf("parsing package path %q failed", pkgPath)
		}
		var subdirBuf bytes.Buffer
		tmpl.Execute(&subdirBuf, cipd.CipdPlatform)
		pkgPath = filepath.Join(jirix.Root, subdirBuf.String())

		info[i] = packageInfoOutput{
			Name:     pkg.Name,
			Path:     pkgPath,
			Version:  pkg.Version,
			Manifest: pkg.ManifestPath,
		}
	}

	for _, i := range info {
		fmt.Printf("* package %s\n", i.Name)
		fmt.Printf("  Path:     %s\n", i.Path)
		fmt.Printf("  Version:  %s\n", i.Version)
		fmt.Printf("  Manifest: %s\n", i.Manifest)
	}

	if jsonOutputFlag != "" {
		if err := writeJSONOutput(info); err != nil {
			return err
		}
	}

	return nil
}
