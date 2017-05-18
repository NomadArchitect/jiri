// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"container/list"
	"fmt"
	glog "log"
	"os"
	"sync"
	"time"

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

type TaskData struct {
	msg      string
	progress int
}

type Task struct {
	taskData *TaskData
	e        *list.Element
	l        *Logger
}

type Logger struct {
	lock                 *sync.Mutex
	LoggerLevel          LogLevel
	goLogger             *glog.Logger
	goErrorLogger        *glog.Logger
	color                color.Color
	progressLines        int
	progressWindowSize   uint
	enableProgress       bool
	progressUpdateNeeded bool
	tasks                *list.List
}

type LogLevel int

const (
	ErrorLevel LogLevel = iota
	WarningLevel
	InfoLevel
	DebugLevel
	TraceLevel
)

func NewLogger(loggerLevel LogLevel, color color.Color, enableProgress bool) *Logger {
	term := os.Getenv("TERM")
	switch term {
	case "dumb", "":
		enableProgress = false
	}
	l := &Logger{
		LoggerLevel:          loggerLevel,
		lock:                 &sync.Mutex{},
		goLogger:             glog.New(os.Stdout, "", 0),
		goErrorLogger:        glog.New(os.Stderr, "", 0),
		color:                color,
		progressLines:        0,
		progressWindowSize:   uint(5),
		enableProgress:       enableProgress,
		progressUpdateNeeded: false,
		tasks:                list.New(),
	}
	if enableProgress {
		go func() {
			for {
				l.repaintProgressMsgs()
				time.Sleep(time.Second / 30)
			}
		}()
	}
	return l
}

func (l *Logger) AddTaskMsg(format string, a ...interface{}) Task {
	if !l.enableProgress {
		return Task{taskData: &TaskData{}}
	}
	t := &TaskData{
		msg:      fmt.Sprintf(format, a...),
		progress: 0,
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	e := l.tasks.PushBack(t)
	l.progressUpdateNeeded = true
	return Task{
		taskData: t,
		e:        e,
		l:        l,
	}
}

func (t *Task) Done() {
	if !t.l.enableProgress {
		return
	}
	t.taskData.progress = 100
	t.l.lock.Lock()
	defer t.l.lock.Unlock()
	t.l.progressUpdateNeeded = true
}

func (l *Logger) repaintProgressMsgs() {
	if !l.enableProgress {
		return
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.progressUpdateNeeded {
		return
	}
	l.clearProgress(0)
	e := l.tasks.Front()
	for i := uint(0); i < l.progressWindowSize; i++ {
		for e != nil {
			if t, ok := e.Value.(*TaskData); ok {
				if t.progress < 100 {
					l.printProgressMsg(t.msg)
					e = e.Next()
					break
				} else {
					temp := e.Next()
					l.tasks.Remove(e)
					e = temp
				}
			} else {
				panic("Control should not come here")
				return
			}
		}
	}
	l.progressUpdateNeeded = false
}

// This is thread unsafe
func (l *Logger) printProgressMsg(msg string) {
	str := fmt.Sprintf("%s: %s\n", l.color.Green("PROGRESS"), msg)
	fmt.Printf(str)
	l.progressLines++
}

// This is thread unsafe
func (l *Logger) clearProgress(t time.Duration) {
	if !l.enableProgress || l.progressLines == 0 {
		return
	}
	buf := ""
	for i := 0; i < l.progressLines; i++ {
		buf = buf + "\033[1A\033[2K\r"
	}
	fmt.Printf(buf)
	l.progressLines = 0
	time.Sleep(t)
}

func (l *Logger) log(prefix, format string, a ...interface{}) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.clearProgress(time.Second / 30)
	l.goLogger.Printf("%s%s", prefix, fmt.Sprintf(format, a...))
}

func (l *Logger) Infof(format string, a ...interface{}) {
	if l.LoggerLevel >= InfoLevel {
		l.log("", format, a...)
	}
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	if l.LoggerLevel >= DebugLevel {
		l.log(l.color.Cyan("DEBUG: "), format, a...)
	}
}

func (l *Logger) Tracef(format string, a ...interface{}) {
	if l.LoggerLevel >= TraceLevel {
		l.log(l.color.Blue("TRACE: "), format, a...)
	}
}

func (l *Logger) Warningf(format string, a ...interface{}) {
	if l.LoggerLevel >= WarningLevel {
		l.log(l.color.Yellow("WARN: "), format, a...)
	}
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	if l.LoggerLevel >= ErrorLevel {
		l.lock.Lock()
		defer l.lock.Unlock()
		l.clearProgress(time.Second / 30)
		l.goErrorLogger.Printf("%s%s", l.color.Red("ERROR: "), fmt.Sprintf(format, a...))
	}
}
