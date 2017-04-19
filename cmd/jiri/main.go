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

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/cmdline"
)

// A Command is an implementation of a jiri command.
type Command struct {
	// Run runs the command.
	// The args are the arguments after the command name.
	Runner func(env *cmdline.Env, cmd *Command, args []string) error

	// Name of the command
	Name string

	// UsageLine is the one-line usage message.
	// It would be auto-generated if blank
	UsageLine string

	// Short is the short description shown in the 'go help' output.
	Short string

	// Long is the long message shown in the 'go help <this-command>' output.
	Long string

	// Flag is a set of flags specific to this command.
	Flags flag.FlagSet

	ArgsName string
	ArgsLong string
}

func (c *Command) UsageErrorf(format string, args ...interface{}) error {
	if format != "" {
		fmt.Fprintf(os.Stderr, "ERROR: ")
		fmt.Fprintf(os.Stderr, format, args...)
		fmt.Fprintf(os.Stderr, "\n")
	}
	c.PrintCmdUsage(os.Stderr)
	return fmt.Errorf("")
}

// Runnable reports whether the command can be run; otherwise
// it is a documentation pseudo-command.
func (c *Command) Runnable() bool {
	return c.Runner != nil
}

// Commands lists the available commands and help topics.
// The order here is the order in which they are printed by 'go help'.
var commands = []*Command{
	cmdBranch,
	cmdGrep,
	cmdImport,
	cmdInit,
	cmdPatch,
	cmdProject,
	cmdProjectConfig,
	cmdRunP,
	cmdSelfUpdate,
	cmdSnapshot,
	cmdStatus,
	cmdUpdate,
	cmdUpload,
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

func mergeFlags(dst, src *flag.FlagSet) {
	src.VisitAll(func(f *flag.Flag) {
		// If there is a collision in flag names, the existing flag in dst wins.
		// Note that flag.Var will panic if it sees a collision.
		if dst.Lookup(f.Name) == nil {
			dst.Var(f.Value, f.Name, f.Usage)
			dst.Lookup(f.Name).DefValue = f.DefValue
		}
	})
}

func copyFlags(src flag.FlagSet) flag.FlagSet {
	var dst flag.FlagSet
	src.VisitAll(func(f *flag.Flag) {
		dst.Var(f.Value, f.Name, f.Usage)
		dst.Lookup(f.Name).DefValue = f.DefValue
	})
	return dst
}

// RunnerFunc is an adapter that turns regular functions into Command.Runner.
// This is similar to Command.RunnerFunc, but the first function argument is
// jiri.X, rather than cmdline.Env.
func RunnerFunc(run func(*jiri.X, []string) error) func(env *cmdline.Env, cmd *Command, args []string) error {
	return func(env *cmdline.Env, cmd *Command, args []string) error {
		x, err := jiri.NewX(env, cmd.UsageErrorf)
		if err != nil {
			return err
		}
		return run(x, args)
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}

	if args[0] == "help" {
		help(args[1:])
		return
	}

	for _, cmd := range commands {
		if cmd.Name == args[0] && cmd.Runnable() {
			env := cmdline.EnvFromOS()
			env.TimerPush("cmdline run")
			defer env.TimerPop()

			// global flags can be passed after command, parse them too
			flags := copyFlags(cmd.Flags)
			flags.Usage = func() {
				cmd.UsageErrorf("")
				os.Exit(1)
			}
			mergeFlags(&flags, flag.CommandLine)
			flags.Parse(args[1:])
			args := flags.Args()

			err := cmd.Runner(env, cmd, args)
			exitCode := 0
			if err != nil {
				err.Error() != "" {
					fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "ERROR\n")
				}
				os.Exit(1)
			}
			os.Exit(0)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "jiri: unknown subcommand %q\nRun 'jiri help' for usage.\n", args[0])
	os.Exit(2)
}

var usageTemplate = `Jiri is a tool for multi-repository development.

Usage:

	jiri command [flags] [arguments]

The commands are:
{{range .}}{{if .Runnable}}
	{{.Name | printf "%-15s"}} {{.Short}}{{end}}{{end}}

Use "jiri help [command]" for more information about a command.

Additional help topics:
{{range .}}{{if not .Runnable}}
	{{.Name | printf "%-15s"}} {{.Short}}{{end}}{{end}}

Use "jiri help [topic]" for more information about that topic.
`

type errWriter struct {
	writer io.Writer
	err    error
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
	os.Exit(1)
}

var helpTemplate = `
{{.Cmd.Long | trim}}

{{if .Cmd.Runnable}}Usage: jiri {{.Cmd.UsageLine}}
{{if .Cmd.ArgsLong}}
  {{.Cmd.ArgsLong}}{{end}}
{{if .CmdFlags}}
jiri {{.Cmd.Name}} flags are:{{range .CmdFlags}}
 -{{.Name}}={{.DefValue}}
    {{.Usage}}{{end}}
{{end}}{{if .GlobalFlags}}
The global flags are:{{range .GlobalFlags}}
 -{{.Name}}={{.DefValue}}
    {{.Usage}}{{end}}
{{end}}{{end}}
`

func help(args []string) {
	if len(args) == 0 {
		printUsage(os.Stdout)
		flag.CommandLine.PrintDefaults()
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: jiri help <command>\n\nToo many arguments given.\n")
		os.Exit(2)
	}

	arg := args[0]

	for _, cmd := range commands {
		if cmd.Name == arg {
			cmd.PrintCmdUsage(os.Stdout)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown help topic %#q.  Run 'jiri help'.\n", arg)
	os.Exit(2)
}

func flagSetAsArray(src *flag.FlagSet, excludeName string) []*flag.Flag {
	var flags []*flag.Flag
	src.VisitAll(func(f *flag.Flag) {
		if excludeName == "" || !strings.HasPrefix(f.Name, excludeName) {
			flags = append(flags, f)
		}
	})
	return flags

}

type UsageData struct {
	GlobalFlags []*flag.Flag
	CmdFlags    []*flag.Flag
	Cmd         *Command
}

func (c *Command) PrintCmdUsage(w io.Writer) {
	globalFlags := flagSetAsArray(flag.CommandLine, "test.")
	cmdFlags := flagSetAsArray(&c.Flags, "")
	if c.UsageLine == "" {
		c.UsageLine = fmt.Sprintf("%s [flags] %s", c.Name, c.ArgsName)
	}
	tmpl(w, helpTemplate, UsageData{globalFlags, cmdFlags, c})
}
