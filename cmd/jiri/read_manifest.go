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

// ReadManifestCallback reads a manifest file from some filepath.
type ReadManifestCallback func(jirix *jiri.X, filepath string) (*project.Manifest, error)

// ReadManifestCommand reads information from a manifest file.
type ReadManifestCommand struct {
	// AttributeName is flag specifying the element attribute= to read.
	AttributeName string

	// ElementName is a flag specifying the name= of the <import> or <project>
	// to search for in the manifest file.
	ElementName string

	// The ReadManifestCallback used by cmdReadManifest.
	readManifestCallback ReadManifestCallback
}

var (
	// The cmdline.Command that Jiri uses in production.
	//
	// This is what gets registered as a subcommand to read manifest files and
	// invoked as the read-manifest command.  Tests create their own instances
	// of ReadManifestCommand to avoid race conditions when attempting to modify
	// the same flags or variables on a shared global instance.
	cmdReadManifest = &cmdline.Command{
		Runner:   jiri.RunnerFunc(readManifestCommand.Run),
		Name:     "read-manifest",
		Short:    "Reads <import> or <project> information from a manifest file",
		ArgsName: "<manifest>",
		ArgsLong: "<manifest> is the manifest file.",
	}

	// The ReadManifestCommand that jiri uses in production.
	//
	// readManifestCmd.Run is wrapped as a jiri.RunnerFunc by cmdReadManifest,
	// instead of being run directly.
	readManifestCommand = &ReadManifestCommand{
		readManifestCallback: project.ManifestFromFile,
	}
)

func init() {
	// Set flags on the global production ReadManifestCommand instance.
	readManifestCommand.SetFlags(&cmdReadManifest.Flags)
}

// SetFlags sets command-line flags for ReadManifestCommand.
func (cmd *ReadManifestCommand) SetFlags(f *flag.FlagSet) {
	f.StringVar(&cmd.ElementName, "element", "",
		"The name= of the <project> or <import>")
	f.StringVar(&cmd.AttributeName, "attribute", "",
		"The element attribute")
}

// Run executes the ReadManifestCommand.
func (cmd *ReadManifestCommand) Run(jirix *jiri.X, args []string) error {
	if len(args) != 1 {
		return jirix.UsageErrorf("Wrong number of args")
	}
	if cmd.ElementName == "" {
		return errors.New("-element is required")
	}
	if cmd.AttributeName == "" {
		return errors.New("-attribute is required")
	}

	manifestPath, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}

	value, err := cmd.readManifest(jirix, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %s", err)
	}

	fmt.Print(value)
	return nil
}

func (cmd *ReadManifestCommand) readManifest(jirix *jiri.X, manifestPath string) (string, error) {
	manifest, err := cmd.readManifestCallback(jirix, manifestPath)
	if err != nil {
		return "", err
	}

	// Check if any <project> elements match the given element name.
	for _, project := range manifest.Projects {
		if project.Name == cmd.ElementName {
			return project.GetAttribute(cmd.AttributeName)
		}
	}

	// Check if any <import> elements match the given element name.
	for _, imprt := range manifest.Imports {
		if imprt.Name == cmd.ElementName {
			return imprt.GetAttribute(cmd.AttributeName)
		}
	}

	// Found nothing.
	return "", fmt.Errorf("found no project/import named %s", cmd.ElementName)
}
