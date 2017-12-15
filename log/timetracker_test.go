// Copyright 2017 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"fuchsia.googlesource.com/jiri/color"
)

const commonPrefix = "seconds taken for operation:"
const timeThreshold = 100 * time.Millisecond

func runTimeTracker(loglevel LogLevel, threshold, sleeptime time.Duration, operations []string) *bytes.Buffer {
	buf := bytes.NewBufferString("")
	logger := NewLogger(loglevel, color.NewColor(color.ColorNever), false, 0, threshold, buf, nil)
	var tts []*TimeTracker
	for _, op := range operations {
		tts = append(tts, logger.TrackTime(op))
	}
	time.Sleep(sleeptime)
	for _, tt := range tts {
		tt.Done()
	}
	return buf
}

func TestTimeTrackerBasic(t *testing.T) {
	t.Parallel()
	buf := runTimeTracker(DebugLevel, timeThreshold, timeThreshold, []string{"new operation"})
	if !strings.Contains(buf.String(), fmt.Sprintf("%s new operation", commonPrefix)) {
		t.Fatalf("logger should have logged timing for this operation")
	}
}

func TestTimeTrackerLogLevel(t *testing.T) {
	t.Parallel()
	buf := runTimeTracker(InfoLevel, timeThreshold, timeThreshold, []string{"new operation"})
	if len(buf.String()) != 0 {
		t.Fatalf("Didnot expect logging, got: %s", buf.String())
	}
}

func TestMultiTimeTracker(t *testing.T) {
	t.Parallel()
	buf := runTimeTracker(DebugLevel, timeThreshold, timeThreshold, []string{"operation 1", "operation 2"})
	if !strings.Contains(buf.String(), fmt.Sprintf("%s operation 1", commonPrefix)) {
		t.Fatalf("logger should have logged timing for operation 1")
	}
	if !strings.Contains(buf.String(), fmt.Sprintf("%s operation 2", commonPrefix)) {
		t.Fatalf("logger should have logged timing for operation 2")
	}
}

func TestTimeTrackerThreshold(t *testing.T) {
	t.Parallel()
	buf := runTimeTracker(DebugLevel, timeThreshold, timeThreshold/2, []string{"operation 1"})
	if len(buf.String()) != 0 {
		t.Fatalf("Didnot expect logging, got: %s", buf.String())
	}
}
