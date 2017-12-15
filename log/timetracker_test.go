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

func TestTimeTrackerBasic(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBufferString("")
	logger := NewLogger(DebugLevel, color.NewColor(color.ColorNever), false, 0, 1, buf, nil)
	tt := logger.TrackTime("new operation")
	time.Sleep(1 * time.Second)
	tt.Done()
	if !strings.Contains(buf.String(), fmt.Sprintf("%s new operation", commonPrefix)) {
		t.Fatalf("logger should have logged timing for this operation")
	}
}

func TestTimeTrackerLogLevel(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBufferString("")
	logger := NewLogger(InfoLevel, color.NewColor(color.ColorNever), false, 0, 1, buf, nil)
	tt := logger.TrackTime("new operation")
	time.Sleep(1 * time.Second)
	tt.Done()
	if len(buf.String()) != 0 {
		t.Fatalf("Didnot expect logging, got: %s", buf.String())
	}
}

func TestMultiTimeTracker(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBufferString("")
	logger := NewLogger(DebugLevel, color.NewColor(color.ColorNever), false, 0, 1, buf, nil)
	tt1 := logger.TrackTime("operation 1")
	tt2 := logger.TrackTime("operation 2")
	time.Sleep(1 * time.Second)
	tt1.Done()
	tt2.Done()
	if !strings.Contains(buf.String(), fmt.Sprintf("%s operation 1", commonPrefix)) {
		t.Fatalf("logger should have logged timing for operation 1")
	}
	if !strings.Contains(buf.String(), fmt.Sprintf("%s operation 2", commonPrefix)) {
		t.Fatalf("logger should have logged timing for operation 2")
	}
}

func TestTimeTrackerThreshold(t *testing.T) {
	t.Parallel()
	buf := bytes.NewBufferString("")
	logger := NewLogger(DebugLevel, color.NewColor(color.ColorNever), false, 0, 1, buf, nil)
	tt := logger.TrackTime("new operation")
	time.Sleep(100 * time.Millisecond)
	tt.Done()
	if len(buf.String()) != 0 {
		t.Fatalf("Didnot expect logging, got: %s", buf.String())
	}
}
