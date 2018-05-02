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

type ReadManifestCallback func(*jiri.X, string) (*project.Manifest, error)

// The actual ReadManifestCallback
var readManifestCallback ReadManifestCallback

// Flag variables for the read-manifest subcommand.
var readManifestFlags struct {
	ElementName   string
	AttributeName string
}

func init() {
	cmdReadManifest.Flags.StringVar(&readManifestFlags.ElementName, "element", "", "The name= of the <project> or <import>")
	cmdReadManifest.Flags.StringVar(&readManifestFlags.AttributeName, "attribute", "", "The element attribute")
	readManifestCallback = project.ManifestFromFile
}

var cmdReadManifest = &cmdline.Command{
	Runner:   jiri.RunnerFunc(runReadManifest),
	Name:     "read-manifest",
	Short:    "Read <import> or <project> information from a manifest",
	Long:     "Reads information about a <project> or <import> from a manifest file.",
	ArgsName: "<manifest>",
	ArgsLong: "<manifest> is the manifest file.",
}

func runReadManifest(jirix *jiri.X, args []string) error {
	// Expect an arg for each flag and one more for the manifest_file.
	if len(args) < 3 {
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
	return "", fmt.Errorf("element %s has no attribute named %s",
		readManifestFlags.ElementName, readManifestFlags.AttributeName)
}
