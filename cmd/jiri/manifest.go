// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/project"
)

// ManifestCommand reads information from a manifest file.
var manifestFlags struct {
	// AttributeName is flag specifying the element attribute= to read.
	AttributeName string

	// ElementName is a flag specifying the name= of the <import> or <project>
	// to search for in the manifest file.
	ElementName string
}

// The cmdline.Command that Jiri uses in production.
var cmdManifest = &cmdline.Command{
	Runner:   jiri.RunnerFunc(runManifest),
	Name:     "manifest",
	Short:    "Reads <import> or <project> information from a manifest file",
	ArgsName: "<manifest>",
	ArgsLong: "<manifest> is the manifest file.",
}

func init() {
	setManifestFlags(&cmdManifest.Flags)
}

// setManifestFlags sets command-line flags for ReadManifestCommand.
func setManifestFlags(f *flag.FlagSet) {
	f.StringVar(&manifestFlags.ElementName, "element", "",
		"The name= of the <project> or <import>")
	f.StringVar(&manifestFlags.AttributeName, "attribute", "",
		"The element attribute")
}

// Run executes the ManifestCommand.
func runManifest(jirix *jiri.X, args []string) error {
	if len(args) != 1 {
		return jirix.UsageErrorf("Wrong number of args")
	}
	if manifestFlags.ElementName == "" {
		return errors.New("-element is required")
	}
	if manifestFlags.AttributeName == "" {
		return errors.New("-attribute is required")
	}

	manifestPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	value, err := readManifest(jirix, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	fmt.Print(value)
	return nil
}

func readManifest(jirix *jiri.X, manifestPath string) (string, error) {
	manifest, err := project.ManifestFromFile(jirix, manifestPath)
	if err != nil {
		return "", err
	}

	// Check if any <project> elements match the given element name.
	for _, project := range manifest.Projects {
		if project.Name == manifestFlags.ElementName {
			return project.GetAttribute(manifestFlags.AttributeName)
		}
	}

	// Check if any <import> elements match the given element name.
	for _, imprt := range manifest.Imports {
		if imprt.Name == manifestFlags.ElementName {
			return imprt.GetAttribute(manifestFlags.AttributeName)
		}
	}

	// Found nothing.
	return "", fmt.Errorf("found no project/import named %s", manifestFlags.ElementName)
}
