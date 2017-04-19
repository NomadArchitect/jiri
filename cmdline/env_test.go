// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cmdline

import (
	"io"
	"os"
	"testing"
)

func writeFunc(s string) func(*Env, io.Writer) {
	return func(_ *Env, w io.Writer) { w.Write([]byte(s)) }
}

func TestEnvWidth(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{"123", 123},
		{"-1", -1},
		{"0", defaultWidth},
		{"", defaultWidth},
		{"foobar", defaultWidth},
	}
	for _, test := range tests {
		// Test using a fake environment.
		env := &Env{Vars: map[string]string{"CMDLINE_WIDTH": test.value}}
		if got, want := env.width(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.value, got, want)
		}
		// Test using the OS environment.
		if err := os.Setenv("CMDLINE_WIDTH", test.value); err != nil {
			t.Errorf("Setenv(%q) failed: %v", test.value, err)
		} else if got, want := EnvFromOS().width(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.value, got, want)
		}
	}
	os.Unsetenv("CMDLINE_WIDTH")
}

func TestEnvStyle(t *testing.T) {
	tests := []struct {
		value string
		want  style
	}{
		{"compact", styleCompact},
		{"full", styleFull},
		{"godoc", styleGoDoc},
		{"", styleCompact},
		{"abc", styleCompact},
		{"foobar", styleCompact},
	}
	for _, test := range tests {
		// Test using a fake environment.
		env := &Env{Vars: map[string]string{"CMDLINE_STYLE": test.value}}
		if got, want := env.style(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.value, got, want)
		}
		// Test using the OS environment.
		if err := os.Setenv("CMDLINE_STYLE", test.value); err != nil {
			t.Errorf("Setenv(%q) failed: %v", test.value, err)
		} else if got, want := EnvFromOS().style(), test.want; got != want {
			t.Errorf("%q got %v, want %v", test.value, got, want)
		}
	}
	os.Unsetenv("CMDLINE_STYLE")
}
