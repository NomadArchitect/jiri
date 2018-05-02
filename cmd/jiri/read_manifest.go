// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
	"fuchsia.googlesource.com/jiri/project"
)

// ReadManifestCallback describes functions that read a manifest file from some
// filepath.
type ReadManifestCallback func(jirix *jiri.X, filepath string) (*project.Manifest, error)

// The ReadManifestCallback used by cmdReadManifest.
var readManifestCallback ReadManifestCallback

// Flags for cmdReadManifest.
var readManifestFlags struct {
	// ElementName specifies the name= attribute of the <import> or <project> to
	// search for in the manifest file.
	ElementName string

	// AttributeName specifies the element attribute= to read.
	AttributeName string
}

func init() {
	cmdReadManifest.Flags.StringVar(&readManifestFlags.ElementName, "element", "",
		"The name= of the <project> or <import>")
	cmdReadManifest.Flags.StringVar(&readManifestFlags.AttributeName, "attribute",
		"", "The element attribute")
	readManifestCallback = project.ManifestFromFile
}

var cmdReadManifest = &cmdline.Command{
	Runner:   jiri.RunnerFunc(runReadManifest),
	Name:     "read-manifest",
	Short:    "Reads <import> or <project> information from a manifest file",
	ArgsName: "<manifest>",
	ArgsLong: "<manifest> is the manifest file.",
}

func runReadManifest(jirix *jiri.X, args []string) error {
	if len(args) != 1 {
		return jirix.UsageErrorf("Wrong number of args")
	}
	if readManifestFlags.ElementName == "" {
		return errors.New("-element is required")
	}
	if readManifestFlags.AttributeName == "" {
		return errors.New("-attribute is required")
	}

	manifestPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	value, err := readManifest(jirix, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %v", err)
	}

	fmt.Fprintf(os.Stdout, value)
	return nil
}

func readManifest(jirix *jiri.X, manifestPath string) (string, error) {
	manifest, err := readManifestCallback(jirix, manifestPath)
	if err != nil {
		return "", err
	}

	// Check if any <project> elements match.
	for _, project := range manifest.Projects {
		if project.Name == readManifestFlags.ElementName {
			return project.GetAttribute(readManifestFlags.AttributeName)
		}
	}

	// Check if any <import> elements match.
	for _, imprt := range manifest.Imports {
		if imprt.Name == readManifestFlags.ElementName {
			return imprt.GetAttribute(readManifestFlags.AttributeName)
		}
	}

	// Found nothing.
	return "", fmt.Errorf("element %s has no attribute named %s", readManifestFlags.ElementName, readManifestFlags.AttributeName)
}
