#!/bin/bash
# Copyright 2016 The Fuchsia Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e

readonly SOURCE="$(cd "$(dirname ${BASH_SOURCE[0]})/.." && pwd)"

cd "${SOURCE}"

GIT_VERSION=$(git describe --always --tags)
BUILD_TIME="$(date --rfc-3339=seconds)"

go build \
	-ldflags "-X \"fuchsia.googlesource.com/jiri/version.Version=${GIT_VERSION}\" -X \"fuchsia.googlesource.com/jiri/version.BuildTime=${BUILD_TIME}\"" \
	fuchsia.googlesource.com/jiri/cmd/jiri
