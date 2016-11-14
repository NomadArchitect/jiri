// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jirilog

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
)

// Logger provides for convenient logging in jiri. It supports logger
// level using global flags. To use it "InitializeGlobalLogger" needs to
// be called once, then GetLogger function can be used to get the logger.
//
// The default logging level is Info. It uses golang logger to log messages internally.
// As an example to use debug logger one needs to run
// jirilog.GetLogger().Debug.Printf(....)
// By default Error logger prints to os.Stderr and others print to os.Stdout.
// Capture function can be used to temporarily capture the logs.
type Logger struct {
	Error        log.Logger
	Info         log.Logger
	Debug        log.Logger
	Trace        log.Logger
	All          log.Logger
	verboseLevel int
}

var (
	DebugVerboseFlag bool
	TraceVerboseFlag bool
	AllVerboseFlag   bool
)

func init() {
	flag.BoolVar(&DebugVerboseFlag, "v", false, "Print debug level output.")
	flag.BoolVar(&TraceVerboseFlag, "vv", false, "Print trace level output.")
	flag.BoolVar(&AllVerboseFlag, "vvv", false, "Print all output.")
}

const (
	InfoLevel  = 0
	DebugLevel = 1
	TraceLevel = 2
	AllLevel   = 3
)

var (
	logger *Logger
)

func GetLogger() Logger {
	if logger == nil {
		panic("Logger not initialized")
	}
	return *logger
}

func InitializeGlobalLogger() {
	loggerLevel := InfoLevel
	if AllVerboseFlag {
		loggerLevel = AllLevel
	} else if TraceVerboseFlag {
		loggerLevel = TraceLevel
	} else if DebugVerboseFlag {
		loggerLevel = DebugLevel
	}
	logger = &Logger{verboseLevel: loggerLevel}
	logger.Error = *(log.New(os.Stderr, "", log.Lmicroseconds))
	discardLogger := log.New(ioutil.Discard, "", 0)
	printLogger := log.New(os.Stdout, "", log.Lmicroseconds)

	if loggerLevel >= InfoLevel {
		logger.Info = *printLogger
	} else {
		logger.Info = *discardLogger
	}

	if loggerLevel >= DebugLevel {
		logger.Debug = *printLogger
	} else {
		logger.Debug = *discardLogger
	}

	if loggerLevel >= TraceLevel {
		logger.Trace = *printLogger
	} else {
		logger.Trace = *discardLogger
	}

	if loggerLevel >= AllLevel {
		logger.All = *printLogger
	} else {
		logger.All = *discardLogger
	}
}

// Capture arranges for the next log to go to supplied io.Writers.
// This will be cleared and not used for any subsequent logs.
// Specifying nil for a writer will result in using the default writer.
// ioutil.Discard should be used to discard output.
func (l Logger) Capture(stdout, stderr io.Writer) Logger {
	if stderr != nil {
		l.Error = *(log.New(stderr, "", log.Lmicroseconds))
	}
	if stdout != nil {
		printLogger := log.New(stdout, "", log.Lmicroseconds)
		if l.verboseLevel >= InfoLevel {
			l.Info = *printLogger
		}

		if l.verboseLevel >= DebugLevel {
			l.Debug = *printLogger
		}

		if l.verboseLevel >= TraceLevel {
			l.Trace = *printLogger
		}

		if l.verboseLevel >= AllLevel {
			logger.All = *printLogger
		}
	}
	return l
}
