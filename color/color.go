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
	BlackFg Color = iota + 30
	RedFg
	GreenFg
	YellowFg
	BlueFg
	MagentaFg
	CyanFg
	WhiteFg
	DefaultFg
)

func colorString(c Color, format string, a ...interface{}) string {
	if !ColorFlag || c == DefaultFg {
		return fmt.Sprintf(format, a...)
	}
	return fmt.Sprintf("%v%vm%v%v", escape, c, fmt.Sprintf(format, a...), clear)
}

func Black(format string, a ...interface{}) string { return colorString(BlackFg, format, a...) }

func Red(format string, a ...interface{}) string { return colorString(RedFg, format, a...) }

func Green(format string, a ...interface{}) string { return colorString(GreenFg, format, a...) }

func Yellow(format string, a ...interface{}) string { return colorString(YellowFg, format, a...) }

func Blue(format string, a ...interface{}) string { return colorString(BlueFg, format, a...) }

func Magenta(format string, a ...interface{}) string { return colorString(MagentaFg, format, a...) }

func Cyan(format string, a ...interface{}) string { return colorString(CyanFg, format, a...) }

func White(format string, a ...interface{}) string { return colorString(WhiteFg, format, a...) }

func DefaultColor(format string, a ...interface{}) string { return colorString(DefaultFg, format, a...) }
