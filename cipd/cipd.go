// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/cmdline"
	"go.fuchsia.dev/jiri/log"
	"go.fuchsia.dev/jiri/retry"
	"go.fuchsia.dev/jiri/version"
	"golang.org/x/sync/semaphore"
)

const (
	cipdBackend            = "https://chrome-infra-packages.appspot.com"
	exitCodeNoValidToken   = 1
	cipdManifestInvalidErr = cmdline.ErrExitCode(25)
)

var (
	// CipdPlatform represents the current runtime platform in cipd platform notation.
	CipdPlatform   Platform
	cipdOS         string
	cipdArch       string
	cipdBinary     string
	selfUpdateOnce sync.Once
	templateRE     = regexp.MustCompile(`\${[^}]*}`)

	// ErrSkipTemplate may be returned from Expander.Expand to indicate that
	// a given expansion doesn't apply to the current template parameters. For
	// example, expanding `"foo/${os=linux,mac}"` with a template parameter of "os"
	// == "win", would return ErrSkipTemplate.
	ErrSkipTemplate = errors.New("package template does not apply to the current system")

	// cipdVersionUntrimmed is the pinned version of the CIPD CLI. The string
	// may contain trailing newlines as a result of being read from a text file.
	//
	// Run `scripts/update_cipd.sh` to update the pin.
	//
	//go:embed cipd_client_version
	cipdVersionUntrimmed string
	//go:embed cipd_client_version.digests
	cipdVersionDigest string
)

func init() {
	cipdOS = runtime.GOOS
	cipdArch = runtime.GOARCH
	if cipdOS == "darwin" {
		cipdOS = "mac"
	}
	if cipdArch == "arm" {
		cipdArch = "armv6l"
	}
	CipdPlatform = Platform{cipdOS, cipdArch}
}

func fetchBinary(jirix *jiri.X, binaryPath, platform, version, digest string) error {
	cipdURL := fmt.Sprintf("%s/client?platform=%s&version=%s", cipdBackend, platform, version)
	data, err := fetchFile(jirix, cipdURL)
	if err != nil {
		return err
	}
	if verified, err := verifyDigest(data, digest); err != nil || !verified {
		if err != nil {
			return err
		}
		return errors.New("cipd failed integrity test")
	}
	// cipd binary verified. Save to disk
	if _, err := os.Stat(filepath.Dir(binaryPath)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory %q for cipd: %v", filepath.Dir(binaryPath), err)
		}
	}
	return writeFile(binaryPath, data)
}

// Bootstrap returns the path of a valid cipd binary. It will fetch cipd from
// remote if a valid cipd binary is not found. It will update cipd if there
// is a new version.
func Bootstrap(jirix *jiri.X, binaryPath string) (string, error) {
	cipdBinary = binaryPath
	bootstrap := func() error {
		// Fetch cipd digest
		cipdDigest, _, err := fetchDigest(CipdPlatform.String())
		if err != nil {
			return err
		}
		if cipdBinary == "" {
			return errors.New("cipd binary path was not set")
		}
		if err != nil {
			return err
		}
		return fetchBinary(jirix, cipdBinary, CipdPlatform.String(), strings.TrimSpace(cipdVersionUntrimmed), cipdDigest)
	}

	getCipd := func() (string, error) {
		if cipdBinary == "" {
			return "", errors.New("cipd binary path was not set")
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

	cipdPath, err := getCipd()
	if err != nil {
		// Could not find cipd binary or cipd is invalid
		// Bootstrap it from scratch
		if err := bootstrap(); err != nil {
			return "", err
		}
		return cipdBinary, nil
	}
	// cipd is found, do self update
	var e error
	selfUpdateOnce.Do(func() {
		e = selfUpdate(cipdPath, strings.TrimSpace(cipdVersionUntrimmed))
	})
	if e != nil {
		// Self update is unsuccessful, redo bootstrap
		if err := bootstrap(); err != nil {
			return "", err
		}
	}
	return cipdPath, nil
}

// FuchsiaPlatform returns a Platform struct which can be used in
// determing the correct path for prebuilt packages. It replace
// the os and arch names from cipd format to a format used by
// Fuchsia developers.
func FuchsiaPlatform(plat Platform) Platform {
	retPlat := Platform{
		OS:   plat.OS,
		Arch: plat.Arch,
	}
	// Currently cipd use "amd64" for x86_64 while fuchsia use "x64",
	// replace "amd64" with "x64".
	// There might be other differences that need to be addressed in
	// the future.
	switch retPlat.Arch {
	case "amd64":
		retPlat.Arch = "x64"
	}
	return retPlat
}

func fetchDigest(platform string) (digest, method string, err error) {
	var digestBuf bytes.Buffer
	digestBuf.Write([]byte(cipdVersionDigest))
	digestScanner := bufio.NewScanner(&digestBuf)
	for digestScanner.Scan() {
		curLine := digestScanner.Text()
		if len(curLine) == 0 || curLine[0] == '#' {
			// Skip comment or empty line
			continue
		}
		fields := strings.Fields(curLine)
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

func selfUpdate(cipdPath, cipdVersion string) error {
	args := []string{"selfupdate", "-version", cipdVersion, "-service-url", cipdBackend}
	command := exec.Command(cipdPath, args...)
	return command.Run()
}

func writeFile(filePath string, data []byte) error {
	tempFile, err := os.CreateTemp(path.Dir(filePath), "cipd.*")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())
	if _, err := tempFile.Write(data); err != nil {
		// Write errors
		return errors.New("I/O error while downloading cipd binary")
	}
	// Set mode to rwxr-xr-x
	if err := tempFile.Chmod(0755); err != nil {
		// Chmod errors
		return errors.New("I/O error while adding executable permission to cipd binary")
	}
	tempFile.Close()
	if err := os.Rename(tempFile.Name(), filePath); err != nil {
		return err
	}
	return nil
}

func verifyDigest(data []byte, cipdDigest string) (bool, error) {
	hash := sha256.Sum256(data)
	hashString := fmt.Sprintf("%x", hash)
	if hashString == strings.ToLower(cipdDigest) {
		return true, nil
	}
	return false, nil
}

func getUserAgent() string {
	ua := "jiri/" + version.GitCommit
	if version.GitCommit == "" {
		ua += "debug"
	}
	return ua
}

func fetchFile(jirix *jiri.X, url string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", getUserAgent())
	var contents []byte
	if err := retry.Function(jirix, func() error {
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("got non-success response: %s", resp.Status)
		}
		contents, err = io.ReadAll(resp.Body)
		return err
	}, fmt.Sprintf("bootstrapping cipd binary"), retry.AttemptsOpt(jirix.Attempts)); err != nil {
		jirix.Logger.Errorf("error: failed to download cipd client: %v\n", err)
		return nil, err
	}
	return contents, nil
}

type packageACL struct {
	path   string
	access bool
}

func checkPackageACL(jirix *jiri.X, cipdPath, jsonDir string) packageACL {
	// cipd should be already bootstrapped before this go routine.
	// Silently return a false just in case if cipd is not found.
	if cipdBinary == "" {
		return packageACL{path: cipdPath, access: false}
	}

	jsonFile, err := os.CreateTemp(jsonDir, "cipd*.json")
	if err != nil {
		jirix.Logger.Warningf("Error while creating temporary file for cipd")
		return packageACL{path: cipdPath, access: false}
	}
	jsonFileName := jsonFile.Name()
	jsonFile.Close()

	args := []string{"acl-check", "-reader", "-json-output", jsonFileName, cipdPath}
	jirix.Logger.Debugf("Invoke cipd with %v", args)

	command := exec.Command(cipdBinary, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf
	// Return false if cipd cannot be executed or output jsonfile contains false.
	if err := command.Run(); err != nil {
		jirix.Logger.Debugf("Error while executing cipd, err: %q, stderr: %q", err, stderrBuf.String())
		return packageACL{path: cipdPath, access: false}
	}

	jsonData, err := os.ReadFile(jsonFileName)
	if err != nil {
		return packageACL{path: cipdPath, access: false}
	}

	var result struct {
		Result bool `json:"result"`
	}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return packageACL{path: cipdPath, access: false}
	}

	if !result.Result {
		return packageACL{path: cipdPath, access: false}
	}

	// Package can be accessed.
	return packageACL{path: cipdPath, access: true}
}

// CheckPackageACL checks cipd's access to packages in map "pkgs". The package
// names in "pkgs" should have trailing '/' removed before calling this
// function.
func CheckPackageACL(jirix *jiri.X, pkgs map[string]bool) error {
	// Not declared as CheckPackageACL(jirix *jiri.X, pkgs map[*package.Package]bool)
	// due to import cycles. Package jiri/package imports jiri/cipd so here we cannot
	// import jiri/package.
	if _, err := Bootstrap(jirix, jirix.CIPDPath()); err != nil {
		return err
	}

	jsonDir, err := os.MkdirTemp("", "jiri_cipd")
	if err != nil {
		return err
	}
	defer os.RemoveAll(jsonDir)

	for key := range pkgs {
		acl := checkPackageACL(jirix, key, jsonDir)
		pkgs[acl.path] = acl.access
	}

	return nil
}

// CheckLoggedIn checks cipd's user login information. It will return true
// if login information is found or return false if login information is not
// found.
func CheckLoggedIn(jirix *jiri.X) (bool, error) {
	cipdPath, err := Bootstrap(jirix, jirix.CIPDPath())
	if err != nil {
		return false, err
	}
	args := []string{"auth-info"}
	command := exec.Command(cipdPath, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf
	if err := command.Run(); err != nil {
		stdErrMsg := strings.TrimSpace(stderrBuf.String())
		jirix.Logger.Debugf("Error happend while executing cipd, err: %q, stderr: %q", err, stdErrMsg)
		if err, ok := err.(*exec.ExitError); ok && err.ExitCode() == exitCodeNoValidToken {
			return false, nil
		}
		return false, fmt.Errorf("failed to check `cipd auth-info`: %w", err)
	}
	return true, nil
}

// Ensure runs cipd binary's ensure functionality over file. Fetched packages will be
// saved to projectRoot directory. Parameter timeout is in minutes.
func Ensure(jirix *jiri.X, file, projectRoot string, timeout uint) error {
	cipdPath, err := Bootstrap(jirix, jirix.CIPDPath())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	args := []string{
		"ensure",
		"-ensure-file", file,
		"-root", projectRoot,
		"-max-threads", strconv.Itoa(jirix.CipdMaxThreads),
	}

	// If jiri is *not* running with -v, use the less verbose cipd "warning"
	// log-level.
	if jirix.Logger.LoggerLevel < log.DebugLevel {
		args = append(args, "-log-level", "warning")
	}

	task := jirix.Logger.AddTaskMsg("Fetching CIPD packages")
	defer task.Done()
	jirix.Logger.Debugf("Invoke cipd with %v", args)

	// Construct arguments and invoke cipd for ensure file
	command := exec.CommandContext(ctx, cipdPath, args...)
	// Add User-Agent info for cipd
	command.Env = append(os.Environ(), "CIPD_HTTP_USER_AGENT_PREFIX="+getUserAgent())
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err = command.Run()
	if ctx.Err() == context.DeadlineExceeded {
		err = ctx.Err()
	}
	return err
}

func EnsureFileVerify(jirix *jiri.X, file string) error {
	cipdPath, err := Bootstrap(jirix, jirix.CIPDPath())
	if err != nil {
		return err
	}
	args := []string{
		"ensure-file-verify",
		"-ensure-file", file,
	}
	// If jiri is *not* running with -v, use the less verbose cipd "warning"
	// log-level.
	if jirix.Logger.LoggerLevel < log.DebugLevel {
		args = append(args, "-log-level", "warning")
	}

	task := jirix.Logger.AddTaskMsg("Verifying CIPD ensure file")
	defer task.Done()
	jirix.Logger.Debugf("Invoke cipd with %v", args)

	// Construct arguments and invoke cipd for ensure file
	command := exec.Command(cipdPath, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	// Add User-Agent info for cipd
	command.Env = append(os.Environ(), "CIPD_HTTP_USER_AGENT_PREFIX="+getUserAgent())
	command.Stdin = os.Stdin
	// Redirect outputs since cipd will print verbose information even
	// if log-level is set to warning
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf

	if err := command.Run(); err != nil {
		jirix.Logger.Errorf("`cipd ensure-file-verify` failed: stdout: %s\nstderr: %s", stdoutBuf.String(), stderrBuf.String())
		return cipdManifestInvalidErr
	}

	return nil
}

// TODO: Using PackageLock in project package directly will cause an import
// cycle. Remove this type once we solve the this issue.

// PackageInstance describes package instance id information generated by cipd
// ensure-file-resolve. It is a copy of PackageLock type in project package.
type PackageInstance struct {
	PackageName string
	VersionTag  string
	InstanceID  string
}

// Resolve runs cipd binary's ensure-file-resolve functionality over file.
// It returns a slice containing resolved packages and cipd instance ids.
func Resolve(jirix *jiri.X, file string) ([]PackageInstance, error) {
	cipdPath, err := Bootstrap(jirix, jirix.CIPDPath())
	if err != nil {
		return nil, err
	}
	args := []string{"ensure-file-resolve", "-ensure-file", file, "-log-level", "warning"}
	jirix.Logger.Debugf("Invoke cipd with %v", args)

	command := exec.Command(cipdPath, args...)
	command.Env = append(os.Environ(), "CIPD_HTTP_USER_AGENT_PREFIX="+getUserAgent())
	var stdoutBuf, stderrBuf bytes.Buffer
	command.Stdin = os.Stdin
	// Redirect outputs since cipd will print verbose information even
	// if log-level is set to warning
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf
	if err := command.Run(); err != nil {
		jirix.Logger.Errorf("cipd returned error: %v", stderrBuf.String())
		return nil, err
	}

	// cipd generates the version file in the same directory of the ensure file
	// if no error is returned
	versionFile := file[:len(file)-len(".ensure")] + ".version"
	defer os.Remove(versionFile)
	return parseVersions(versionFile)
}

func parseVersions(file string) ([]PackageInstance, error) {
	versionReader, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer versionReader.Close()
	versionScanner := bufio.NewScanner(versionReader)
	// An example cipd version looks like:
	// ==========================================================
	// # Do not modify manually. All changes will be overwritten.
	// fuchsia/clang/linux-amd64
	// 	git_revision:280fa3c2d2ddb0b5dcb31113c0b1c2259982b7e7
	// 	eRoGS8qgx370QAIRgLDmbhpdPey8ti47B2Z3LMzwcXQC
	//
	// fuchsia/clang/mac-amd64
	// 	git_revision:280fa3c2d2ddb0b5dcb31113c0b1c2259982b7e7
	// 	BQhlnpoWG081CyLzA0zB1vCr8YPdb2DO2jnYe3Lsw4oC
	// ===========================================================
	// Parse version file using DFA

	const (
		stWaitingPkg = "a package name"
		stWaitingVer = "a package version"
		stWaitingIID = "an instance ID"
		stWaitingNL  = "a new line"
	)

	state := stWaitingPkg
	pkg := ""
	ver := ""
	iid := ""
	lineNo := 0
	makeError := func(fmtStr string, args ...interface{}) error {
		args = append([]interface{}{lineNo}, args...)
		return fmt.Errorf("failed to parse versions file (line %d): "+fmtStr, args...)
	}
	output := make([]PackageInstance, 0)
	for versionScanner.Scan() {
		lineNo++
		line := strings.TrimSpace(versionScanner.Text())
		// Comments are grammatically insignificant (unlike empty lines), so skip
		// the completely.
		if len(line) > 0 && line[0] == '#' {
			continue
		}

		switch state {
		case stWaitingPkg:
			if line == "" {
				continue // can have more than one empty line between triples
			}
			pkg = line
			state = stWaitingVer

		case stWaitingVer:
			if line == "" {
				return nil, makeError("expecting a version name, not a new line")
			}
			ver = line
			state = stWaitingIID

		case stWaitingIID:
			if line == "" {
				return nil, makeError("expecting an instance ID, not a new line")
			}
			iid = line
			output = append(output, PackageInstance{pkg, ver, iid})
			pkg, ver, iid = "", "", ""
			state = stWaitingNL

		case stWaitingNL:
			if line == "" {
				state = stWaitingPkg
				continue
			}
			return nil, makeError("expecting an empty line between each version definition triple")
		}
	}
	return output, nil
}

type packageFloatingRef struct {
	pkg      PackageInstance
	err      error
	floating bool
}

// CheckFloatingRefs determines if pkgs contains a floating ref which shouldn't
// be used normally.
func CheckFloatingRefs(jirix *jiri.X, pkgs map[PackageInstance]bool, plats map[PackageInstance][]Platform) error {
	if _, err := Bootstrap(jirix, jirix.CIPDPath()); err != nil {
		return err
	}

	jsonDir, err := os.MkdirTemp("", "jiri_cipd")
	if err != nil {
		return err
	}
	defer os.RemoveAll(jsonDir)

	c := make(chan packageFloatingRef)
	sem := semaphore.NewWeighted(10)
	var errBuf bytes.Buffer
	for k := range pkgs {
		plat, ok := plats[k]
		if !ok {
			return fmt.Errorf("Platforms for package \"%s\" is not found", k.PackageName)
		}
		go checkFloatingRefs(jirix, k, plat, jsonDir, sem, c)
	}

	for i := 0; i < len(pkgs); i++ {
		floatingRef := <-c
		pkgs[floatingRef.pkg] = floatingRef.floating
		if floatingRef.err != nil {
			errBuf.WriteString(fmt.Sprintf("error happened while checking package %q with version %q: %v\n", floatingRef.pkg.PackageName, floatingRef.pkg.VersionTag, floatingRef.err.Error()))
		}
	}

	if errBuf.Len() != 0 {
		// Remote trailing '\n'
		errBuf.Truncate(errBuf.Len() - 1)
		return errors.New(errBuf.String())
	}
	return nil
}

type describeJSON struct {
	Refs []refsJSON `json:"refs,omitempty"`
}

type refsJSON struct {
	Ref string `json:"ref,omitempty"`
}

func checkFloatingRefs(jirix *jiri.X, pkg PackageInstance, plats []Platform, jsonDir string, sem *semaphore.Weighted, c chan<- packageFloatingRef) {
	// cipd should already bootstrapped before calling
	// this function.
	sem.Acquire(context.Background(), 1)
	defer sem.Release(1)
	if cipdBinary == "" {
		c <- packageFloatingRef{
			pkg:      pkg,
			err:      errors.New("cipd is not bootstrapped when calling checkFloatingRefs"),
			floating: false,
		}
		return
	}
	// jsonFile will be cleaned up by caller.
	jsonFile, err := os.CreateTemp(jsonDir, "cipd*.json")
	if err != nil {
		c <- packageFloatingRef{
			pkg:      pkg,
			err:      err,
			floating: false,
		}
		return
	}
	jsonFileName := jsonFile.Name()
	jsonFile.Close()

	// Remove ${platform}, ${os} ... from package name before calling cipd describe
	// as it will fail when these tags are not compatible with current host.
	pkgName := pkg.PackageName
	if MustExpand(pkgName) {
		expandedPkgName, err := Expand(pkgName, plats)
		if err != nil {
			c <- packageFloatingRef{
				pkg:      pkg,
				err:      err,
				floating: false,
			}
			return
		}
		if len(expandedPkgName) == 0 {
			c <- packageFloatingRef{
				pkg: pkg,
				// avoid using %q as we don't want escape characters in the output.
				err:      fmt.Errorf("cannot expand package \"%s\"", pkgName),
				floating: false,
			}
			return
		}
		pkgName = expandedPkgName[0]
	}

	args := []string{"describe", pkgName, "-version", pkg.VersionTag, "-json-output", jsonFileName}
	jirix.Logger.Debugf("Invoke cipd with %v", args)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	command := exec.Command(cipdBinary, args...)
	command.Env = append(os.Environ(), "CIPD_HTTP_USER_AGENT_PREFIX="+getUserAgent())
	command.Stdin = os.Stdin
	command.Stdout = &stdoutBuf
	command.Stderr = &stderrBuf

	if err := command.Run(); err != nil {
		c <- packageFloatingRef{
			pkg:      pkg,
			err:      fmt.Errorf("cipd describe failed due to error: %v, stdout: %s\n, stderr: %s", err, stdoutBuf.String(), stderrBuf.String()),
			floating: false,
		}
		return
	}

	jsonData, err := os.ReadFile(jsonFileName)
	if err != nil {
		c <- packageFloatingRef{
			pkg:      pkg,
			err:      err,
			floating: false,
		}
		return
	}
	// Example of generated JSON:
	// {
	// 	"result": {
	// 	  "pin": {
	// 		"package": "gn/gn/linux-amd64",
	// 		"instance_id": "4usiirrra6WbnCKgplRoiJ8EcAsCuqCOd_7tpf_yXrAC"
	// 	  },
	// 	  "registered_by": "user:infra-internal-gn-builder@chops-service-accounts.iam.gserviceaccount.com",
	// 	  "registered_ts": 1554328925,
	// 	  "refs": [
	// 		{
	// 		  "ref": "latest",
	// 		  "instance_id": "4usiirrra6WbnCKgplRoiJ8EcAsCuqCOd_7tpf_yXrAC",
	// 		  "modified_by": "user:infra-internal-gn-builder@chops-service-accounts.iam.gserviceaccount.com",
	// 		  "modified_ts": 1554328926
	// 		}
	// 	  ],
	// 	  "tags": [
	// 		{
	// 		  "tag": "git_repository:https://gn.googlesource.com/gn",
	// 		  "registered_by": "user:infra-internal-gn-builder@chops-service-accounts.iam.gserviceaccount.com",
	// 		  "registered_ts": 1554328925
	// 		},
	// 		{
	// 		  "tag": "git_revision:64b846c96daeb3eaf08e26d8a84d8451c6cb712b",
	// 		  "registered_by": "user:infra-internal-gn-builder@chops-service-accounts.iam.gserviceaccount.com",
	// 		  "registered_ts": 1554328925
	// 		}
	// 	  ]
	// 	}
	// }
	// Only "refs" is needed.

	var result struct {
		Result describeJSON `json:"result"`
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		c <- packageFloatingRef{
			pkg:      pkg,
			err:      err,
			floating: false,
		}
		return
	}

	for _, v := range result.Result.Refs {
		if v.Ref == pkg.VersionTag {
			c <- packageFloatingRef{pkg: pkg, err: nil, floating: true}
			return
		}
	}
	c <- packageFloatingRef{pkg: pkg, err: nil, floating: false}
	return
}

// Platform contains the parameters for a "${platform}" template.
// The string value can be obtained by calling String().
type Platform struct {
	// OS defines the operating system of this platform. It can be any OS
	// supported by golang.
	OS string
	// Arch defines the CPU architecture of this platform. It can be any
	// architecture supported by golang.
	Arch string
}

// NewPlatform parses a platform string into Platform struct.
func NewPlatform(s string) (Platform, error) {
	fields := strings.Split(s, "-")
	if len(fields) != 2 {
		return Platform{"", ""}, fmt.Errorf("illegal platform %q", s)
	}
	return Platform{fields[0], fields[1]}, nil
}

// String generates a string represents the Platform in "OS"-"Arch" form.
func (p Platform) String() string {
	return p.OS + "-" + p.Arch
}

// Expander returns an Expander populated with p's fields.
func (p Platform) Expander() Expander {
	return Expander{
		"os":       p.OS,
		"arch":     p.Arch,
		"platform": p.String(),
	}
}

// Expander is a mapping of simple string substitutions which is used to
// expand cipd package name templates. For example:
//
//	ex, err := template.Expander{
//	  "platform": "mac-amd64"
//	}.Expand("foo/${platform}")
//
// `ex` would be "foo/mac-amd64".
type Expander map[string]string

// Expand applies package template expansion rules to the package template,
//
// If err == ErrSkipTemplate, that means that this template does not apply to
// this os/arch combination and should be skipped.
//
// The expansion rules are as follows:
//   - "some text" will pass through unchanged
//   - "${variable}" will directly substitute the given variable
//   - "${variable=val1,val2}" will substitute the given variable, if its value
//     matches one of the values in the list of values. If the current value
//     does not match, this returns ErrSkipTemplate.
//
// Attempting to expand an unknown variable is an error.
// After expansion, any lingering '$' in the template is an error.
func (t Expander) Expand(template string) (pkg string, err error) {
	skip := false

	pkg = templateRE.ReplaceAllStringFunc(template, func(parm string) string {
		// ${...}
		contents := parm[2 : len(parm)-1]

		varNameValues := strings.SplitN(contents, "=", 2)
		if len(varNameValues) == 1 {
			// ${varName}
			if value, ok := t[varNameValues[0]]; ok {
				return value
			}

			err = fmt.Errorf("unknown variable in ${%s}", contents)
		}

		// ${varName=value,value}
		ourValue, ok := t[varNameValues[0]]
		if !ok {
			err = fmt.Errorf("unknown variable %q", parm)
			return parm
		}

		for _, val := range strings.Split(varNameValues[1], ",") {
			if val == ourValue {
				return ourValue
			}
		}
		skip = true
		return parm
	})
	if skip {
		err = ErrSkipTemplate
	}
	if err == nil && strings.ContainsRune(pkg, '$') {
		err = fmt.Errorf("unable to process some variables in %q", template)
	}
	return
}

// Expand method expands a cipdPath that contains templates such as ${platform}
// into concrete full paths. It might return an empty slice if platforms
// do not match the requirements in cipdPath.
func Expand(cipdPath string, platforms []Platform) ([]string, error) {
	output := make([]string, 0)
	//expanders := make([]Expander, 0)
	if !MustExpand(cipdPath) {
		output = append(output, cipdPath)
		return output, nil
	}

	for _, plat := range platforms {
		pkg, err := plat.Expander().Expand(cipdPath)
		if err == ErrSkipTemplate {
			continue
		}
		if err != nil {
			return nil, err
		}
		output = append(output, pkg)
	}
	return output, nil
}

// Decl method expands a cipdPath that contains ${platform}, ${os}, ${arch}
// with information in platforms. Unlike the Expand method which
// returns a list of expanded cipd paths, the Decl method only returns a
// single path containing all platforms. For example, if platforms contain
// "linux-amd64" and "linux-arm64", ${platform} will be replaced to
// ${platform=linux-amd64,linux-arm64}. This is a workaround for a limitation
// in 'cipd ensure-file-resolve' which requires the header of '.ensure' file
// to contain all available platforms. But in some cases, a package may miss
// a particular platform, which will cause a crash on this cipd command. By
// explicitly list all supporting platforms in the cipdPath, we can avoid
// crashing cipd.
func Decl(cipdPath string, platforms []Platform) (string, error) {
	if !MustExpand(cipdPath) || len(platforms) == 0 {
		return cipdPath, nil
	}

	osMap := make(map[string]bool)
	platMap := make(map[string]bool)
	archMap := make(map[string]bool)

	replacedOS := "${os="
	replacedArch := "${arch="
	replacedPlat := "${platform="

	for _, plat := range platforms {
		if _, ok := osMap[plat.OS]; !ok {
			replacedOS += plat.OS + ","
			osMap[plat.OS] = true
		}
		if _, ok := archMap[plat.Arch]; !ok {
			replacedArch += plat.Arch + ","
			archMap[plat.Arch] = true
		}
		if _, ok := platMap[plat.String()]; !ok {
			replacedPlat += plat.String() + ","
			platMap[plat.String()] = true
		}
	}
	replacedOS = replacedOS[:len(replacedOS)-1] + "}"
	replacedArch = replacedArch[:len(replacedArch)-1] + "}"
	replacedPlat = replacedPlat[:len(replacedPlat)-1] + "}"

	cipdPath = strings.Replace(cipdPath, "${os}", replacedOS, -1)
	cipdPath = strings.Replace(cipdPath, "${arch}", replacedArch, -1)
	cipdPath = strings.Replace(cipdPath, "${platform}", replacedPlat, -1)
	return cipdPath, nil
}

// MustExpand checks if template usages such as "${platform}" exist
// in cipdPath. If they exist, this function will return true. Otherwise
// it returns false.
func MustExpand(cipdPath string) bool {
	return templateRE.MatchString(cipdPath)
}

// DefaultPlatforms returns a slice of Platform objects that are currently
// validated by jiri.
func DefaultPlatforms() []Platform {
	return []Platform{
		{"linux", "amd64"},
		{"mac", "amd64"},
	}
}
