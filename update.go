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

const JiriHost = "https://fuchsia.googlesource.com/jiri"

func Update() error {
	commit, err := getCurrentCommit()
	if err != nil {
		return nil
	}
	if commit != version.GitCommit {
		b, err := downloadFile(commit)
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

func getCurrentCommit() (string, error) {
	// Use Gitiles to find out the latest revision.
	url := fmt.Sprintf("%s/+log/master?n=1", JiriHost)
	var body io.Reader
	method, body := "GET", nil
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return "", fmt.Errorf("NewRequest(%q, %q, %v) failed: %v", method, url, body, err)
	}
	req.Header.Add("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Do(%v) failed: %v", req, err)
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Query:Do(%v) failed: %v", req, res.StatusCode)
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
		return "", fmt.Errorf("failed to decode the response: %v", err)
	}
	if len(result.Log) == 0 {
		return "", fmt.Errorf("no log entries")
	}

	return result.Log[0].Commit, nil
}

func downloadFile(version string) ([]byte, error) {
	//version = "8db27b125a72c5b27da73b9d08f6376f80f48ce7"

	url := fmt.Sprintf("https://storage.googleapis.com/fuchsia-build/jiri/%v-%v/%v", runtime.GOOS, runtime.GOARCH, version)
	res, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Get(%v) failed: %v", url, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad HTTP status: %v", res.StatusCode)
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
	
	oldfile, err := ioutil.TempFile(dir, "jiri")
	if err != nil {
		return err
	}
	defer os.Remove(oldfile.Name())
	
	if err := oldfile.Close(); err != nil {
		return err
	}

	// Backup the existing version.
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
