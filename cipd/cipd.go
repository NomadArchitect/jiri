// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/osutil"
	"fuchsia.googlesource.com/jiri/version"
)

const (
	cipdBackend          = "https://chrome-infra-packages.appspot.com"
	cipdVersionURL       = "https://fuchsia.googlesource.com/buildtools/+/master/.cipd_version"
	cipdVersionDigestURL = "https://fuchsia.googlesource.com/buildtools/+/master/.cipd_version.digests"
	pkgNotFoundStr       = "No matching packages."
)

var (
	cipdVersionFetchErr = errors.New("failed to fetch cipd version file")
)

func bootstrapCipdImpl(cipdPath, cipdPlatform, cipdVersion, cipdDigest, hashMethod string) error {
	cipdURL := fmt.Sprintf("%s/client?platform=%s&version=%s", cipdBackend, cipdPlatform, cipdVersion)
	cipdBinary, err := fetchFile(cipdURL)
	if err != nil {
		return err
	}
	if verified, err := verifyDigest(cipdBinary, cipdDigest, hashMethod); err != nil || !verified {
		if err != nil {
			return err
		}
		return errors.New("cipd failed integrity test")
	}
	// cipd binary verified. Save to disk
	return saveOrReplaceCipd(cipdPath, cipdBinary)
}

func boostrapCipd() error {
	cipdPlatform, err := determinePlatform()
	if err != nil {
		return err
	}
	// Fetch cipd version
	cipdVersionBytes, err := fetchFileFromGitiles(cipdVersionURL)
	if err != nil {
		return err
	}
	// Fetch cipd digest
	cipdDigest, hashMethod, err := fetchCipdDigest(cipdPlatform)
	if err != nil {
		return err
	}
	// Remove tailing LF
	cipdVersion := strings.Trim(string(cipdVersionBytes), "\n ")
	cipdPath, err := getCipdPath()
	if err != nil {
		return err
	}
	return bootstrapCipdImpl(cipdPath, cipdPlatform, cipdVersion, cipdDigest, hashMethod)
}

func fetchCipdDigest(platform string) (digest, method string, err error) {
	cipdDigestBytes, err := fetchFileFromGitiles(cipdVersionDigestURL)
	if err != nil {
		return "", "", err
	}
	var digestBuf bytes.Buffer
	digestBuf.Write(cipdDigestBytes)
	digestScanner := bufio.NewScanner(&digestBuf)
	for digestScanner.Scan() {
		curLine := digestScanner.Text()
		if len(curLine) > 0 && curLine[0] == '#' {
			// Skip comment line
			continue
		}
		fields := strings.Fields(curLine)
		if len(fields) == 0 {
			// Skip empty line
			continue
		}
		if len(fields) != 3 {
			return "", "", errors.New("unsupported cipd digest file format")
		}
		if fields[0] == platform {
			digest = fields[2]
			method = fields[1]
			err = nil
			return
		}
	}
	return "", "", errors.New("no matching platform found in cipd digest file")
}

func cipdSelfUpdate() error {
	cipdBinary, err := getAndCheckCipdPath()
	if err != nil {
		return err
	}
	cipdVersionBytes, err := fetchFileFromGitiles(cipdVersionURL)
	if err != nil {
		return cipdVersionFetchErr
	}
	cipdVersion := string(cipdVersionBytes)
	return cipdSelfUpdateImpl(cipdBinary, cipdVersion)
}

func cipdSelfUpdateImpl(cipdBinary, cipdVersion string) error {
	args := []string{"selfupdate", "-version", cipdVersion, "-service-url", cipdBackend}
	command := exec.Command(cipdBinary, args...)
	return command.Run()
}

func saveOrReplaceCipd(cipdPath string, data []byte) error {
	tempFile, err := ioutil.TempFile(path.Dir(cipdPath), "cipd.*")
	if err != nil {
		return err
	}
	n, err := tempFile.Write(data)
	// Set mode to rwxr-xr-x
	tempFile.Chmod(0755)
	if err != nil || n != len(data) {
		// Write errors
		tempFile.Close()
		os.Remove(tempFile.Name())
		return errors.New("I/O error while downloading cipd binary")
	}
	tempFile.Close()
	if err := os.Rename(tempFile.Name(), cipdPath); err != nil {
		os.Remove(tempFile.Name())
		return err
	}
	return nil
}

func verifyDigest(data []byte, cipdDigest, hashMethod string) (bool, error) {
	if hashMethod != "sha256" {
		return false, fmt.Errorf("hash method %q is not supported yet", hashMethod)
	}
	hash := sha256.Sum256(data)
	hashString := fmt.Sprintf("%x", hash)
	if hashString == strings.ToLower(cipdDigest) {
		return true, nil
	}
	return false, nil
}

func determinePlatform() (string, error) {
	hostPlatform := runtime.GOOS
	hostArch := runtime.GOARCH

	switch hostPlatform {
	case "linux":
	case "darwin":
		hostPlatform = "mac"
	default:
		return "", fmt.Errorf("unsupported operating system %q for fetching cipd binary", hostPlatform)
	}
	switch hostArch {
	case "amd64", "arm64":
	case "arm":
		hostArch = "armv6l"
	default:
		return "", fmt.Errorf("unsupported machine architecture %q for fetching cipd binary", hostArch)
	}
	return hostPlatform + "-" + hostArch, nil
}

func userAgent() string {
	ua := "jiri/" + version.GitCommit
	if version.GitCommit == "" {
		ua += "debug"
	}
	return ua
}
func fetchFile(url string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func fetchFileFromGitiles(url string) ([]byte, error) {
	url += "?format=TEXT"
	// Fetch and decode data
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent())
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Decode the file on the lfy
	base64Decoder := base64.NewDecoder(base64.StdEncoding, resp.Body)
	return ioutil.ReadAll(base64Decoder)
}

func getCipdPath() (string, error) {
	jiriPath, err := osutil.Executable()
	if err != nil {
		return "", err
	}
	// Assume cipd binary is located in the same directory of jiri
	jiriBinaryRoot := path.Dir(jiriPath)
	cipdBinary := path.Join(jiriBinaryRoot, "cipd")
	return cipdBinary, nil
}

func getAndCheckCipdPath() (string, error) {
	cipdBinary, err := getCipdPath()
	if err != nil {
		return "", err
	}
	fileInfo, err := os.Stat(cipdBinary)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("cipd binary was not found at %q", cipdBinary)
		}
		return "", err
	}
	// Check if cipd binary has execution permission
	if fileInfo.Mode()&0111 == 0 {
		return "", fmt.Errorf("cipd binary at %q is not executable", cipdBinary)
	}
	return cipdBinary, nil

}

func getPrefix(path string) string {
	// trailing '/' should be trimmed before calling
	// this function
	return path[:strings.LastIndexByte(path, '/')]
}

// resolvePkgPath replace ${platform}, ${os}, ${arch} in path
// with runtime GOOS and GOARCH values
func resolvePkgPath(path string) string {
	path = strings.Replace(path, "${platform}", "${os}-${arch}", -1)

	os := runtime.GOOS
	if os == "darwin" {
		os = "mac"
	}
	arch := runtime.GOARCH
	if arch == "arm" {
		arch = "armv6l"
	}

	path = strings.Replace(path, "${os}", os, -1)
	path = strings.Replace(path, "${arch}", arch, -1)
	return path
}

func returnCheckPrefixResults(rtnList []bool, commu chan bool) {
	for _, val := range rtnList {
		commu <- val
	}
}

func checkPackagesWithSamePrefix(jirix *jiri.X, paths []string, commu chan bool) {
	if len(paths) == 0 {
		return
	}
	// Sent back results in all cases
	rtnList := make([]bool, len(paths))
	defer returnCheckPrefixResults(rtnList, commu)

	// cipd should be bootstrapped by caller
	// silent return just in case if cipd is
	// not found
	cipdPath, err := getCipdPath()
	if err != nil {
		return
	}
	// Map from path to its index in paths
	// In rare cases, "example_package/${platform}" and
	// "example_package/linux-amd64" may both exist in paths.
	// We use []int to resolve index collisions
	pathIndex := make(map[string][]int)
	for idx, currStr := range paths {
		currStr = resolvePkgPath(currStr)
		pathIndex[currStr] = append(pathIndex[currStr], idx)
	}

	// Run cipd command
	prefix := getPrefix(paths[0])
	args := []string{"ls", prefix}
	if jirix != nil {
		jirix.Logger.Debugf("Invoke cipd with %v", args)
	}
	command := exec.Command(cipdPath, args...)
	var stdoutBuf bytes.Buffer
	command.Stdout = &stdoutBuf
	// Silent return if cipd cannot be executed
	if err := command.Run(); err != nil {
		return
	}
	// Parse cipd output
	stdoutScanner := bufio.NewScanner(&stdoutBuf)
	for stdoutScanner.Scan() {
		currLine := strings.Trim(stdoutScanner.Text(), "\n\r ")
		if currLine == pkgNotFoundStr {
			// All packages not exist under this prefix
			return
		}
		if idxList, ok := pathIndex[currLine]; ok {
			for _, idx := range idxList {
				rtnList[idx] = true
			}
		}
	}
}

// CheckPackageACL will check if cipd has access to packages
// in map "pkgs". The package names in "pkgs" should have trailing '/'
// removed before calling this function
func CheckPackageACL(jirix *jiri.X, pkgs map[string]bool) error {
	if err := cipdSelfUpdate(); err != nil {
		// Allow cipd execution if cipd binary exists but version file is not available
		if err != cipdVersionFetchErr {
			// Self update failure, do bootstrap
			err = boostrapCipd()
			if err != nil {
				return err
			}
		}
	}

	// Find common prefix for all pkgs
	prefixMap := make(map[string][]string)
	for key := range pkgs {
		prefix := getPrefix(key)
		val, _ := prefixMap[prefix]
		prefixMap[prefix] = append(val, key)
	}

	// Distribute work
	chanMap := make(map[string]chan bool)
	for key, val := range prefixMap {
		currChan := make(chan bool)
		chanMap[key] = currChan
		go checkPackagesWithSamePrefix(jirix, val, currChan)
	}

	for key, val := range prefixMap {
		currChan := chanMap[key]
		for i := 0; i < len(val); i++ {
			pkgs[val[i]] = <-currChan
		}
	}

	return nil
}

// Ensure runs cipd binary's ensure funcationality over file. Fetched packages will be
// saved to projectRoot directory. Parameter timeout is in minitues
func Ensure(jirix *jiri.X, file, projectRoot string, timeout uint) error {
	if err := cipdSelfUpdate(); err != nil {
		// Allow cipd execution if cipd binary exists but version file is not available
		if err != cipdVersionFetchErr {
			// Self update failure, do bootstrap
			err = boostrapCipd()
			if err != nil {
				return err
			}
		}
	}
	cipdBinary, err := getAndCheckCipdPath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	args := []string{"ensure", "-ensure-file", file, "-root", projectRoot, "-log-level", "warning"}
	// Walkaround to avoid cycle import in cipd_test.go
	if jirix != nil {
		jirix.Logger.Debugf("Invoke cipd with %v", args)
	}
	// Construct arguments and invoke cipd for ensure file
	command := exec.CommandContext(ctx, cipdBinary, args...)
	// Add User-Agent info for cipd
	command.Env = append(os.Environ(), "CIPD_HTTP_USER_AGENT_PREFIX="+userAgent())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}
