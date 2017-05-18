// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"container/list"
	"fmt"
	glog "log"
	"os"
	"strconv"
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

type Tasks struct {
	lastid   int64
	taskMap  map[string]*list.Element
	taskList *list.List
}

type Logger struct {
	lock                   *sync.Mutex
	LoggerLevel            LogLevel
	goLogger               *glog.Logger
	goErrorLogger          *glog.Logger
	color                  color.Color
	previousProgressMsgLen int
	enableProgress         bool
	tasks                  *Tasks
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
		LoggerLevel:   loggerLevel,
		lock:          &sync.Mutex{},
		goLogger:      glog.New(os.Stdout, "", 0),
		goErrorLogger: glog.New(os.Stderr, "", 0),
		color:         color,
		previousProgressMsgLen: 0,
		enableProgress:         enableProgress,
		tasks: &Tasks{
			taskMap:  make(map[string]*list.Element),
			taskList: list.New(),
			lastid:   int64(-1),
		},
	}
	if enableProgress {
		go func() {
			for {
				l.printTasksMsgs()
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}
	return l
}

func (l *Logger) AddTaskMsg(format string, a ...interface{}) string {
	if !l.enableProgress {
		return "dummy"
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	l.tasks.lastid++
	id := strconv.FormatInt(l.tasks.lastid, 10)
	l.tasks.taskMap[id] = l.tasks.taskList.PushFront(fmt.Sprintf(format, a...))
	return id
}

func (l *Logger) RemoveTaskMsg(taskId string) {
	if !l.enableProgress {
		return
	}
	l.lock.Lock()
	defer l.lock.Unlock()
	if e, ok := l.tasks.taskMap[taskId]; ok {
		l.tasks.taskList.Remove(e)
		delete(l.tasks.taskMap, taskId)
	}
}

func (l *Logger) printTasksMsgs() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.tasks.taskList.Len() != 0 {
		if str, ok := l.tasks.taskList.Front().Value.(string); ok {
			l.progressMsg(str)
		}
	}

}

// This is thread unsafe
func (l *Logger) progressMsg(msg string) {
	if !l.enableProgress {
		return
	}
	str := fmt.Sprintf("%s: %s", l.color.Green("PROGRESS"), msg)
	if l.previousProgressMsgLen != 0 {
		fmt.Printf("\r%*s\r", l.previousProgressMsgLen, "")
	}
	fmt.Printf(str)
	l.previousProgressMsgLen = len(str)
}

func (l *Logger) log(prefix, format string, a ...interface{}) {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.previousProgressMsgLen != 0 {
		fmt.Printf("\r%*s\r", l.previousProgressMsgLen, "")
		l.previousProgressMsgLen = 0
	}
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
		if l.previousProgressMsgLen != 0 {
			fmt.Printf("\r%*s\r", l.previousProgressMsgLen, "")
			l.previousProgressMsgLen = 0
		}
		l.goErrorLogger.Printf("%s%s", l.color.Red("ERROR: "), fmt.Sprintf(format, a...))
	}
}
