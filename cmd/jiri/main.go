// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"syscall"
	"text/template"
)

// A Command is an implementation of a jiri command.
type Command struct {
	// Run runs the command.
	// The args are the arguments after the command name.
	Runner func(cmd *Command, args []string) error

	// UsageLine is the one-line usage message.
	// The first word in the line is taken to be the command name.
	UsageLine string

	// Short is the short description shown in the 'go help' output.
	Short string

	// Long is the long message shown in the 'go help <this-command>' output.
	Long string

	// Flag is a set of flags specific to this command.
	Flags flag.FlagSet

	// CustomFlags indicates that the command will do its own
	// flag parsing.
	CustomFlags bool
}

func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

func (c *Command) Usage() {
	fmt.Fprintf(os.Stderr, "usage: %s\n\n", c.UsageLine)
	fmt.Fprintf(os.Stderr, "%s\n", strings.TrimSpace(c.Long))
	os.Exit(2)
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command.
func (c *Command) Runnable() bool {
	return c.Runner != nil
}

// Commands lists the available commands and help topics.
// The order here is the order in which they are printed by 'go help'.
var commands = []*Command{
	cmdImport,
	cmdProject,
	cmdRunP,
	cmdSnapshot,
	cmdUpdate,
	cmdVersion,

	helpFilesystem,
	helpManifest,
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	if runtime.GOOS == "darwin" {
		var rLimit syscall.Rlimit
		err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			fmt.Println("Unable to obtain rlimit: ", err)
		}
		if rLimit.Cur < rLimit.Max {
			rLimit.Max = 999999
			rLimit.Cur = 999999
			err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
			if err != nil {
				fmt.Println("Unable to increase rlimit: ", err)
			}
		}
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()
	log.SetFlags(0)
	
	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	if args[0] == "help" {
		help(args[1:])
		return
	}

	for _, cmd := range commands {
		if cmd.Name() == args[0] && cmd.Runnable() {
			cmd.Flags.Usage = func() {
				cmd.Usage()
			}
			if cmd.CustomFlags {
				args = args[1:]
			} else {
				cmd.Flags.Parse(args[1:])
				args = cmd.Flags.Args()
			}
			cmd.Runner(cmd, args)
			os.Exit(0)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "jiri: unknown subcommand %q\nRun 'jiri help' for usage.\n", args[0])
	os.Exit(2)
}

var usageTemplate = `Jiri is a tool for multi-repository development.

Usage:

	jiri command [arguments]

The commands are:
{{range .}}{{if .Runnable}}
	{{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "jiri help [command]" for more information about a command.

Additional help topics:
{{range .}}{{if not .Runnable}}
	{{.Name | printf "%-11s"}} {{.Short}}{{end}}{{end}}

Use "jiri help [topic]" for more information about that topic.
`

type errWriter struct {
	writer	io.Writer
	err		error
}

func (e *errWriter) Write(b []byte) (int, error) {
	n, err := e.writer.Write(b)
	if err != nil {
		e.err = err
	}
	return n, err
}

func tmpl(w io.Writer, text string, data interface{}) {
	t := template.New("top")
	t.Funcs(template.FuncMap{"trim": strings.TrimSpace, "capitalize": strings.ToTitle})
	template.Must(t.Parse(text))
	ew := &errWriter{writer: w}
	err := t.Execute(ew, data)
	if ew.err != nil {
		if strings.Contains(ew.err.Error(), "pipe") {
			os.Exit(1)
		}
		log.Fatalf("writing output: %v", ew.err)
	}
	if err != nil {
		panic(err)
	}
}

func printUsage(w io.Writer) {
	bw := bufio.NewWriter(w)
	tmpl(bw, usageTemplate, commands)
	bw.Flush()
}

func usage() {
	printUsage(os.Stderr)
	os.Exit(2)
}

var helpTemplate = `{{if .Runnable}}usage: jiri {{.UsageLine}}

{{end}}{{.Long | trim}}
`

func help(args []string) {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: jiri help command\n\nToo many arguments given.\n")
		os.Exit(2)
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name() == arg {
			tmpl(os.Stdout, helpTemplate, cmd)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'jiri help'.\n", arg)
	os.Exit(2)
}
