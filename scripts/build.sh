#!/usr/bin/env bash
# Copyright 2017 The Fuchsia Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly GIT_DIR="$(dirname "${SCRIPT_DIR}")"

readonly PKG_PATH="fuchsia.googlesource.com/jiri"
readonly GIT2GO_VENDOR="${GIT_DIR}/vendor/github.com/libgit2/git2go/vendor"

BORINGSSL_SRC="${GIT2GO_VENDOR}/boringssl"
BORINGSSL_BUILD="${GIT2GO_VENDOR}/boringssl/build"
mkdir -p -- "${BORINGSSL_BUILD}"
pushd "${BORINGSSL_BUILD}"
[[ -f "${BORINGSSL_BUILD}/build.ninja" ]] || cmake -GNinja -DCMAKE_C_FLAGS=-fPIC ..
ninja
popd

LIBSSH2_SRC="${GIT2GO_VENDOR}/libssh2"
LIBSSH2_BUILD="${GIT2GO_VENDOR}/libssh2/build"
mkdir -p -- "${LIBSSH2_BUILD}"
pushd "${LIBSSH2_BUILD}"
[[ -f "${LIBSSH2_BUILD}/build.ninja" ]] || cmake -GNinja \
  -DBUILD_SHARED_LIBS=OFF \
  -DENABLE_ZLIB_COMPRESSION=ON \
  -DCMAKE_INSTALL_LIBDIR=lib \
  -DBUILD_EXAMPLES=OFF \
  -DBUILD_TESTING=OFF \
  -DCRYPTO_BACKEND=OpenSSL \
  -DOPENSSL_INCLUDE_DIR="${BORINGSSL_SRC}/include" \
  -DOPENSSL_SSL_LIBRARY="${BORINGSSL_BUILD}/ssl/libssl.a" \
  -DOPENSSL_CRYPTO_LIBRARY="${BORINGSSL_BUILD}/crypto/libcrypto.a" \
  ..
ninja
popd

CURL_SRC="${GIT2GO_VENDOR}/curl"
CURL_BUILD="${GIT2GO_VENDOR}/curl/build"
mkdir -p -- "${CURL_BUILD}"
pushd "${CURL_BUILD}"
[[ -f "${CURL_BUILD}/build.ninja" ]] || cmake -GNinja \
  -DBUILD_CURL_EXE=OFF \
  -DBUILD_TESTING=OFF \
  -DCURL_STATICLIB=ON \
  -DHTTP_ONLY=ON \
  -DCMAKE_USE_OPENSSL=ON \
  -DCMAKE_USE_LIBSSH2=OFF \
  -DENABLE_UNIX_SOCKETS=OFF \
  -DOPENSSL_INCLUDE_DIR="${BORINGSSL_SRC}/include" \
  -DOPENSSL_SSL_LIBRARY="${BORINGSSL_BUILD}/ssl/libssl.a" \
  -DOPENSSL_CRYPTO_LIBRARY="${BORINGSSL_BUILD}/crypto/libcrypto.a" \
  -DHAVE_OPENSSL_ENGINE_H=OFF \
  ..
ninja
popd

LIBGIT2_SRC="${GIT2GO_VENDOR}/libgit2"
LIBGIT2_BUILD="${GIT2GO_VENDOR}/libgit2/build"
mkdir -p "${LIBGIT2_BUILD}"
pushd "${LIBGIT2_BUILD}"
[[ -f "${LIBGIT2_BUILD}/build.ninja" ]] || cmake -GNinja \
  -DCMAKE_BUILD_TYPE=RelWithDebInfo \
  -DCMAKE_C_FLAGS=-fPIC \
  -DTHREADSAFE=ON \
  -DBUILD_CLAR=OFF \
  -DBUILD_SHARED_LIBS=OFF \
  -DOPENSSL_INCLUDE_DIR="${BORINGSSL_SRC}/include" \
  -DOPENSSL_LIBRARIES="${BORINGSSL_BUILD}/ssl/libssl.a" \
  -DCURL_INCLUDE_DIRS="${CURL_SRC}/include;${CURL_BUILD}/include/curl" \
  -DCURL_LIBRARIES="${CURL_BUILD}/libcurl.a" \
  ..
ninja
popd

# Build Jiri
readonly GO_DIR="$(cd ../../.. && pwd)"
GOPATH="${GO_DIR}" go build -ldflags "-X \"fuchsia.googlesource.com/jiri/version.GitCommit=${GIT_COMMIT}\" -X \"fuchsia.googlesource.com/jiri/version.BuildTime=${BUILD_TIME}\"" -a -o "jiri" "${PKG_PATH}/cmd/jiri"
