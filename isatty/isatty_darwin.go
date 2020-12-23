// Copyright 2017 The Fuchsia Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// +build darwin

package isatty

import "syscall"

const ioctlTermios = syscall.TIOCGETA
