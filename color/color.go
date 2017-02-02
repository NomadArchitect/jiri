// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package color

import (
	"flag"
	"fmt"
	"os"
)

var (
	ColorFlag bool
)

type Color int
type Colorfn func(format string, a ...interface{}) string

func init() {
	flag.BoolVar(&ColorFlag, "color", true, "Use color to format output.")
}

func InitializeGlobalColors() {
	if ColorFlag {
		term := os.Getenv("TERM")
		switch term {
		case "dumb", "":
			ColorFlag = false
			fmt.Println("Warning: your terminal doesn't support colors")
		}
	}
}

const (
	escape = "\033["
	clear  = escape + "0m"
)

// Foreground text colors
const (
	Blackfg Color = iota + 30
	Redfg
	Greenfg
	Yellowfg
	Bluefg
	Magentafg
	Cyanfg
	Whitefg
	Defaultfg
)

func colorString(c Color, format string, a ...interface{}) string {
	if !ColorFlag || c == Defaultfg {
		return fmt.Sprintf(format, a...)
	}
	return fmt.Sprintf("%v%vm%v%v", escape, c, fmt.Sprintf(format, a...), clear)
}

func Black(format string, a ...interface{}) string { return colorString(Blackfg, format, a...) }

func Red(format string, a ...interface{}) string { return colorString(Redfg, format, a...) }

func Green(format string, a ...interface{}) string { return colorString(Greenfg, format, a...) }

func Yellow(format string, a ...interface{}) string { return colorString(Yellowfg, format, a...) }

func Blue(format string, a ...interface{}) string { return colorString(Bluefg, format, a...) }

func Magenta(format string, a ...interface{}) string { return colorString(Magentafg, format, a...) }

func Cyan(format string, a ...interface{}) string { return colorString(Cyanfg, format, a...) }

func White(format string, a ...interface{}) string { return colorString(Whitefg, format, a...) }

func DefaultColor(format string, a ...interface{}) string { return colorString(Defaultfg, format, a...) }
