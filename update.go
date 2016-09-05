// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jiri

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"fuchsia.googlesource.com/jiri/osutil"
	"fuchsia.googlesource.com/jiri/version"
)

const (
	JiriRepository = "https://fuchsia.googlesource.com/jiri"
	JiriStorageBucket = "https://storage.googleapis.com/fuchsia-build/jiri"
)

// Update checks whether a new version of Jiri is available and if so,
// it will download it and replace the current version with the new one.
func Update() error {
	commit, err := getCurrentCommit(JiriRepository)
	if err != nil {
		return nil
	}
	if commit != version.GitCommit {
		b, err := downloadFile(JiriStorageBucket, commit)
		if err != nil {
			return nil
		}
		err = updateExecutable(b)
		if err != nil {
			return err
		}
	}
	return nil
}

func getCurrentCommit(repository string) (string, error) {
	url := fmt.Sprintf("%s/+log/master?n=1", repository)
	// Use Gitiles to find out the latest revision.
	var body io.Reader
	method, body := "GET", nil
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed: %v", http.StatusText(res.StatusCode))
	}
	defer res.Body.Close()

	r := bufio.NewReader(res.Body)

	// The first line of the input is the XSSI guard ")]}'".
	if _, err := r.ReadSlice('\n'); err != nil {
		return "", err
	}

	result := struct {
		Log []struct {
			Commit string `json:"commit"`
		} `json:"log"`
	}{}

	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Log) == 0 {
		return "", fmt.Errorf("no log entries")
	}

	return result.Log[0].Commit, nil
}

func downloadFile(bucket, version string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s-%s/%s", bucket, runtime.GOOS, runtime.GOARCH, version)
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed: %v", http.StatusText(res.StatusCode))
	}
	defer res.Body.Close()

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func updateExecutable(b []byte) error {
	path, err := osutil.Executable()
	if err != nil {
		return err
	}

	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	// Write the new version to a file.
	newfile, err := ioutil.TempFile(dir, "jiri")
	if err != nil {
		return err
	}

	if _, err := newfile.Write(b); err != nil {
		return err
	}

	if err := newfile.Chmod(fi.Mode()); err != nil {
		return err
	}

	if err := newfile.Close(); err != nil {
		return err
	}

	// Backup the existing version.
	oldfile, err := ioutil.TempFile(dir, "jiri")
	if err != nil {
		return err
	}
	defer os.Remove(oldfile.Name())

	if err := oldfile.Close(); err != nil {
		return err
	}

	err = os.Rename(path, oldfile.Name())
	if err != nil {
		return err
	}

	// Replace the existing version.
	err = os.Rename(newfile.Name(), path)
	if err != nil {
		// Try to rollback the change in case of error.
		rerr := os.Rename(oldfile.Name(), path)
		if rerr != nil {
			return rerr
		}
		return err
	}

	return nil
}
