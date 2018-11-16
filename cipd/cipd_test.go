// Copyright 2018 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cipd

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

const (
	// Some random valid cipd version tags from infra/tools/cipd
	cipdVersionForTestA = "git_revision:00e2d8b49a4e7505d1c71f19d15c9e7c5b9245a5"
	cipdVersionForTestB = "git_revision:8fac632847b1ce0de3b57d16d0f2193625f4a4f0"
	// Digest generated by cipd selfupdate-roll ...
	cipdDigestForTestLinuxAMD64A  = "df37ffc2588e345a31ca790d773b6136fedbd2efbf9a34cb735dd34b6891c16c"
	cipdDigestForTestLinuxARM64A  = "650f2a045f8587062a16299a650aa24ba5c5c0652585a2d9bd56594369d5f99e"
	cipdDigestForTestLinuxARMv6lA = "61b657c860ddc39d3286ced073c843852b1dafc0222af0bdc22ad988b289d733"
	cipdDigestForTestMacAMD64A    = "4d015791ed6f03f305cf6a5a673a447e5c47ff5fdb701f43f99fba9ca73e61f8"
	cipdDigestForTestLinuxAMD64B  = "bdc971fd2895c3771e0709d2a3ec5fcace69c59a3a9f9dc33ab76fbc2f777d40"
	cipdDigestForTestLinuxARM64B  = "e1d6aadc9bfc155e9088aa3de39b9d3311c7359f398f372b5ad1c308e25edfeb"
	cipdDigestForTestLinuxARMv6lB = "3ad97b47ecc1b358c8ebd1b0307087d354433d88f24bf8ece096fb05452837f9"
	cipdDigestForTestMacAMD64B    = "167edadf7c7c019a40b9f7869a4c05b2d9834427dad68e295442ef9ebce88dba"
)

func retrieveDigestA(platform string) string {
	switch platform {
	case "linux-amd64":
		return cipdDigestForTestLinuxAMD64A
	case "linux-arm64":
		return cipdDigestForTestLinuxARM64A
	case "linux-armv6l":
		return cipdDigestForTestLinuxARMv6lA
	case "mac-amd64":
		return cipdDigestForTestMacAMD64A
	}
	return ""
}

func retrieveDigestB(platform string) string {
	switch platform {
	case "linux-amd64":
		return cipdDigestForTestLinuxAMD64B
	case "linux-arm64":
		return cipdDigestForTestLinuxARM64B
	case "linux-armv6l":
		return cipdDigestForTestLinuxARMv6lB
	case "mac-amd64":
		return cipdDigestForTestMacAMD64B
	}
	return ""
}

func TestSupportedPlatform(t *testing.T) {
	if cipdPlatform == "" {
		t.Fatal("Unknown platform")
	}
}

func TestBootstrapCipdImplForAllPlatform(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "jiri-test")
	if err != nil {
		t.Fatal("failed to creat temp dir for testing")
	}
	defer os.RemoveAll(tmpDir)
	// Testing cipdVersionForTestA
	testBootstrapCipdImpl(tmpDir, "linux-amd64", cipdVersionForTestA, cipdDigestForTestLinuxAMD64A, t)
	testBootstrapCipdImpl(tmpDir, "linux-arm64", cipdVersionForTestA, cipdDigestForTestLinuxARM64A, t)
	testBootstrapCipdImpl(tmpDir, "linux-armv6l", cipdVersionForTestA, cipdDigestForTestLinuxARMv6lA, t)
	testBootstrapCipdImpl(tmpDir, "mac-amd64", cipdVersionForTestA, cipdDigestForTestMacAMD64A, t)
	// Testing cipdVersionForTestB
	testBootstrapCipdImpl(tmpDir, "linux-amd64", cipdVersionForTestB, cipdDigestForTestLinuxAMD64B, t)
	testBootstrapCipdImpl(tmpDir, "linux-arm64", cipdVersionForTestB, cipdDigestForTestLinuxARM64B, t)
	testBootstrapCipdImpl(tmpDir, "linux-armv6l", cipdVersionForTestB, cipdDigestForTestLinuxARMv6lB, t)
	testBootstrapCipdImpl(tmpDir, "mac-amd64", cipdVersionForTestB, cipdDigestForTestMacAMD64B, t)

}

func testBootstrapCipdImpl(tmpDir, cipdPlatform, cipdVersion, cipdDigest string, t *testing.T) {
	cipdPath := path.Join(tmpDir, "cipd"+cipdPlatform+cipdVersion)
	if err := bootstrapCipdImpl(cipdPath, cipdPlatform, cipdVersion, cipdDigest); err != nil {
		t.Fatalf("failed to retrieve cipd binary for platform %q on version %q with digest %q: %v", cipdPlatform, cipdVersion, cipdDigest, err)
	}
}

func TestFetchCipdVersion(t *testing.T) {
	// Assume cipd version is always a git commit hash for now
	versionStr := string(cipdVersion)
	if len(versionStr) != len("git_revision:00e2d8b49a4e7505d1c71f19d15c9e7c5b9245a5") ||
		versionStr[:len("git_revision:")] != "git_revision:" {
		t.Fatalf("unsupported cipd version tag: %q", versionStr)
	}
	versionHash := versionStr[len("git_revision:"):]
	if _, err := hex.DecodeString(versionHash); err != nil {
		t.Fatalf("unsupported cipd version tag: %q", versionStr)
	}
}

func TestFetchCipdDigest(t *testing.T) {
	digest, _, err := fetchCipdDigest("linux-amd64")
	if err != nil {
		t.Fatalf("failed to retrieve cipd digest: %v", err)
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatalf("digest is not a valid hex string: %q", digest)
	}
}

func TestCipdSelfUpdateImpl(t *testing.T) {

	tmpDir, err := ioutil.TempDir("", "jiri-test")
	if err != nil {
		t.Fatal("failed to creat temp dir for testing")
	}
	defer os.RemoveAll(tmpDir)
	// Bootstrap cipd to version A
	cipdPath := path.Join(tmpDir, "cipd")
	if err := bootstrapCipdImpl(cipdPath, cipdPlatform, cipdVersionForTestA, retrieveDigestA(cipdPlatform)); err != nil {
		t.Fatalf("failed to bootstrap cipd with version %q: %v", cipdVersionForTestA, err)
	}
	// Perform cipd self update to version B
	if err := cipdSelfUpdateImpl(cipdPath, cipdVersionForTestB); err != nil {
		t.Fatalf("failed to perform cipd self update: %v", err)
	}
	// Verify self updated cipd
	cipdData, err := ioutil.ReadFile(cipdPath)
	if err != nil {
		t.Fatalf("failed to read self-updated cipd binary: %v", err)
	}
	verified, err := verifyDigest(cipdData, retrieveDigestB(cipdPlatform))
	if err != nil {
		t.Fatal(err)
	}
	if !verified {
		t.Fatalf("self-updated cipd failed integrity test")
	}
}

func TestEnsure(t *testing.T) {
	if err := boostrapCipd(); err != nil {
		t.Fatal(err)
	}
	cipdPath, err := getAndCheckCipdPath()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cipdPath)
	// Write test ensure file
	testEnsureFile, err := ioutil.TempFile("", "test_jiri*.ensure")
	if err != nil {
		t.Fatalf("failed to create test ensure file: %v", err)
	}
	defer testEnsureFile.Close()
	defer os.Remove(testEnsureFile.Name())
	_, err = testEnsureFile.Write([]byte(`
$ParanoidMode CheckPresence

# GN
gn/gn/${platform} git_revision:bdb0fd02324b120cacde634a9235405061c8ea06
`))
	if err != nil {
		t.Fatalf("failed to write test ensure file: %v", err)
	}
	testEnsureFile.Sync()
	tmpDir, err := ioutil.TempDir("", "jiri-test")
	if err != nil {
		t.Fatal("failed to creat temp dir for testing")
	}
	defer os.RemoveAll(tmpDir)
	// Invoke Ensure on test ensure file
	if err := Ensure(nil, testEnsureFile.Name(), tmpDir, 30); err != nil {
		t.Fatal(err)
	}
	// Check the existence downloaded package
	gnPath := path.Join(tmpDir, "gn")
	if _, err := os.Stat(gnPath); err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("fetched cipd package is not found at %q", gnPath)
		}
		t.Fatal(err)
	}
}
