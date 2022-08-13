// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command gendoc generates godoc comments describing the usage of tools based
// on the cmdline package.
//
// Usage:
//   go run gendoc.go [flags] <pkg> [args]
//
// <pkg> is the package path for the tool.
//
// [args] are the arguments to pass to the tool to produce usage output.  If no
// args are given, runs "<tool> help ..."
//
// The reason this command is located under a testdata directory is to enforce
// its idiomatic use via "go run".
//
// The gendoc command itself is not based on the cmdline library to avoid
// non-trivial bootstrapping.  In particular, if the compilation of gendoc
// requires GOPATH to contain the vanadium Go workspaces, then running the
// gendoc command requires the jiri tool, which in turn may depend on the gendoc
// command.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	flagEnv     string
	flagInstall string
	flagOut     string
	flagTags    string
)

func main() {
	flag.StringVar(&flagEnv, "env", "os", `Environment variables to set before running command.  If "os", grabs vars from the underlying OS.  If empty, doesn't set any vars.  Otherwise vars are expected to be comma-separated entries of the form KEY1=VALUE1,KEY2=VALUE2,...`)
	flag.StringVar(&flagInstall, "install", "", "Comma separated list of packages to install before running command.  All commands that are built will be on the PATH.")
	flag.StringVar(&flagOut, "out", "./doc.go", "Path to the output file.")
	flag.StringVar(&flagTags, "tags", "", "Tags for go build, also added as build constraints in the generated output file.")
	flag.Parse()
	if err := generate(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(args []string) error {
	if got, want := len(args), 1; got < want {
		return fmt.Errorf("gendoc requires at least one argument\nusage: gendoc <pkg> [args]")
	}
	pkg, args := args[0], args[1:]

	// Find out the binary name from the pkg name.
	var listOut bytes.Buffer
	listCmd := exec.Command("go", "list", pkg)
	listCmd.Stdout = &listOut
	if err := listCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n%v\n", strings.Join(listCmd.Args, " "), err, listOut.String())
	}
	binName := filepath.Base(strings.TrimSpace(listOut.String()))

	// Install all packages in a temporary directory.
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("TempDir() failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	pkgs := []string{pkg}
	if flagInstall != "" {
		pkgs = append(pkgs, strings.Split(flagInstall, ",")...)
	}
	for _, installPkg := range pkgs {
		installArgs := []string{"go", "install", "-tags=" + flagTags, installPkg}
		installCmd := exec.Command("jiri", installArgs...)
		installCmd.Env = append(os.Environ(), "GOBIN="+tmpDir)
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("%q failed: %v\n", strings.Join(installCmd.Args, " "), err)
		}
	}

	// Run the binary to generate documentation.
	var out bytes.Buffer
	if len(args) == 0 {
		args = []string{"help", "..."}
	}
	runCmd := exec.Command(filepath.Join(tmpDir, binName), args...)
	runCmd.Stdout = &out
	runCmd.Env = runEnviron(tmpDir)
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("%q failed: %v\n%v\n", strings.Join(runCmd.Args, " "), err, out.String())
	}
	var tagsConstraint string
	if flagTags != "" {
		tagsConstraint = fmt.Sprintf("// +build %s\n\n", flagTags)
	}
	doc := fmt.Sprintf(`// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

%s/*
%s*/
package main
`, tagsConstraint, suppressParallelFlag(out.String()))

	// Write the result to the output file.
	path, perm := flagOut, os.FileMode(0644)
	if err := os.WriteFile(path, []byte(doc), perm); err != nil {
		return fmt.Errorf("WriteFile(%v, %v) failed: %v\n", path, perm, err)
	}
	return nil
}

// suppressParallelFlag replaces the default value of the test.parallel flag
// with the literal string "<number of threads>". The default value of the
// test.parallel flag is GOMAXPROCS, which (since Go1.5) is set to the number
// of logical CPU threads on the current system. This causes problems with the
// vanadium-go-generate test, which requires that the output of gendoc is the
// same on all systems.
func suppressParallelFlag(input string) string {
	pattern := regexp.MustCompile("(?m:(^ -test\\.parallel=)(?:\\d)+$)")
	return pattern.ReplaceAllString(input, "$1<number of threads>")
}

// runEnviron returns the environment variables to use when running the command
// to retrieve full help information.
func runEnviron(binDir string) []string {
	// Never return nil, which signals exec.Command to use os.Environ.
	in, out := strings.Split(flagEnv, ","), make([]string, 0)
	if flagEnv == "os" {
		in = os.Environ()
	}
	updatedPath := false
	for _, e := range in {
		if e == "" {
			continue
		}
		if strings.HasPrefix(e, "PATH=") {
			e = "PATH=" + binDir + string(os.PathListSeparator) + e[5:]
			updatedPath = true
		}
		out = append(out, e)
	}
	if !updatedPath {
		out = append(out, "PATH="+binDir)
	}
	out = append(out, "CMDLINE_STYLE=godoc")
	return out
}
