// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"flag"
	"io"
	glog "log"
	"os"
	"sync"

	"fuchsia.googlesource.com/jiri/color"
)

// Logger provides for convenient logging in jiri. It supports logger
// level using global flags. To use it "InitializeGlobalLogger" needs to
// be called once, then GetLogger function can be used to get the logger or
// log functions can be called directly
//
// The default logging level is Info. It uses golang logger to log messages internally.
// As an example to use debug logger one needs to run
// log.GetLogger().Debugf(....)
// or
// log.Debugf(....)
// By default Error logger prints to os.Stderr and others print to os.Stdout.
// Capture function can be used to temporarily capture the logs.
type Logger struct {
	lock          *sync.Mutex
	loggerLevel   LogLevel
	goLogger      *glog.Logger
	goErrorLogger *glog.Logger
}

var (
	DebugVerboseFlag          bool
	TraceVerboseFlag          bool
	AllVerboseFlag            bool
	UserActionableVerboseFlag bool
)

func init() {
	flag.BoolVar(&UserActionableVerboseFlag, "quiet", false, "Only print user actionable messages.")
	flag.BoolVar(&DebugVerboseFlag, "v", false, "Print debug level output.")
	flag.BoolVar(&TraceVerboseFlag, "vv", false, "Print trace level output.")
	flag.BoolVar(&AllVerboseFlag, "vvv", false, "Print all output.")
}

type LogLevel int

const (
	ErrorLevel LogLevel = iota
	InfoLevel
	DebugLevel
	TraceLevel
	AllLevel
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
	if UserActionableVerboseFlag {
		loggerLevel = ErrorLevel
	} else if AllVerboseFlag {
		loggerLevel = AllLevel
	} else if TraceVerboseFlag {
		loggerLevel = TraceLevel
	} else if DebugVerboseFlag {
		loggerLevel = DebugLevel
	}
	logger = &Logger{
		loggerLevel:   loggerLevel,
		lock:          &sync.Mutex{},
		goLogger:      glog.New(os.Stdout, "", glog.Lmicroseconds),
		goErrorLogger: glog.New(os.Stderr, "", glog.Lmicroseconds),
	}
}

// Capture arranges for the next log to go to supplied io.Writers.
// This will be cleared and not used for any subsequent logs.
// Specifying nil for a writer will result in using the default writer.
// ioutil.Discard should be used to discard output.
func (l Logger) Capture(stdout, stderr io.Writer) Logger {
	if stdout != nil {
		l.goLogger = glog.New(stdout, "", glog.Lmicroseconds)
	}
	if stderr != nil {
		l.goErrorLogger = glog.New(stderr, "", glog.Lmicroseconds)
	}
	return l
}

func (l Logger) log(colorfn color.Colorfn, format string, a ...interface{}) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.goLogger.Printf(colorfn(format, a...))
}

func Infof(format string, a ...interface{}) {
	GetLogger().Infof(format, a...)
}

func (l Logger) Infof(format string, a ...interface{}) {
	if l.loggerLevel >= InfoLevel {
		l.log(color.DefaultColor, format, a...)
	}
}

func Debugf(format string, a ...interface{}) {
	GetLogger().Debugf(format, a...)
}

func (l Logger) Debugf(format string, a ...interface{}) {
	if l.loggerLevel >= DebugLevel {
		l.log(color.Yellow, format, a...)
	}
}

func Tracef(format string, a ...interface{}) {
	GetLogger().Tracef(format, a...)
}

func (l Logger) Tracef(format string, a ...interface{}) {
	if l.loggerLevel >= TraceLevel {
		l.log(color.Blue, format, a...)
	}
}

func Logf(format string, a ...interface{}) {
	GetLogger().Logf(format, a...)
}

func (l Logger) Logf(format string, a ...interface{}) {
	if l.loggerLevel >= AllLevel {
		l.log(color.DefaultColor, format, a...)
	}
}

func Errorf(format string, a ...interface{}) {
	GetLogger().Errorf(format, a...)
}

func (l Logger) Errorf(format string, a ...interface{}) {
	if l.loggerLevel >= ErrorLevel {
		l.lock.Lock()
		defer l.lock.Unlock()
		l.goErrorLogger.Printf(color.Red(format, a...))
	}
}
