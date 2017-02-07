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

type ColorWrapper interface {
	Black(format string, a ...interface{}) string
	Red(format string, a ...interface{}) string
	Green(format string, a ...interface{}) string
	Yellow(format string, a ...interface{}) string
	Blue(format string, a ...interface{}) string
	Magenta(format string, a ...interface{}) string
	Cyan(format string, a ...interface{}) string
	White(format string, a ...interface{}) string
	DefaultColor(format string, a ...interface{}) string
}
var c ColorWrapper

type color struct{}

func (color) Black(format string, a ...interface{}) string { return colorString(BlackFg, format, a...) }
func (color) Red(format string, a ...interface{}) string { return colorString(RedFg, format, a...) }
func (color) Green(format string, a ...interface{}) string { return colorString(GreenFg, format, a...) }
func (color) Yellow(format string, a ...interface{}) string { return colorString(YellowFg, format, a...) }
func (color) Blue(format string, a ...interface{}) string { return colorString(BlueFg, format, a...) }
func (color) Magenta(format string, a ...interface{}) string { return colorString(MagentaFg, format, a...) }
func (color) Cyan(format string, a ...interface{}) string { return colorString(CyanFg, format, a...) }
func (color) White(format string, a ...interface{}) string { return colorString(WhiteFg, format, a...) }
func (color) DefaultColor(format string, a ...interface{}) string { return colorString(DefaultFg, format, a...) }

func colorString(c Color, format string, a ...interface{}) string {
	if c == DefaultFg {
		return fmt.Sprintf(format, a...)
	}
	return fmt.Sprintf("%v%vm%v%v", escape, c, fmt.Sprintf(format, a...), clear)
}

type noColor struct{}

func (noColor) Black(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Red(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Green(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Yellow(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Blue(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Magenta(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) Cyan(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) White(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }
func (noColor) DefaultColor(format string, a ...interface{}) string { return fmt.Sprintf(format, a...) }


func InitializeGlobalColors() {
	if ColorFlag {
		term := os.Getenv("TERM")
		switch term {
		case "dumb", "":
			ColorFlag = false
			fmt.Println("Warning: your terminal doesn't support colors")
		}
	}

	if ColorFlag {
		c = color{}
	} else {
		c = noColor{}
	}
}


func Black(format string, a ...interface{}) string { return c.Black(format, a...) }
func Red(format string, a ...interface{}) string { return c.Red(format, a...) }
func Green(format string, a ...interface{}) string { return c.Green(format, a...) }
func Yellow(format string, a ...interface{}) string { return c.Yellow(format, a...) }
func Blue(format string, a ...interface{}) string { return c.Blue(format, a...) }
func Magenta(format string, a ...interface{}) string { return c.Magenta(format, a...) }
func Cyan(format string, a ...interface{}) string { return c.Cyan(format, a...) }
func White(format string, a ...interface{}) string { return c.White(format, a...) }
func DefaultColor(format string, a ...interface{}) string { return c.DefaultColor(format, a...) }
