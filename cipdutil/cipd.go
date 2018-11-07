package cipdutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"time"

	"fuchsia.googlesource.com/jiri"
	"fuchsia.googlesource.com/jiri/osutil"
)

// Assume cipd binary is located in the same directory
// of jiri
func getCipdPath() (string, error) {
	jiriPath, err := osutil.Executable()
	if err != nil {
		return "", err
	}

	jiriBinaryRoot := path.Dir(jiriPath)
	cipdBinary := path.Join(jiriBinaryRoot, "cipd")
	if _, err := os.Stat(cipdBinary); os.IsNotExist(err) {
		return "", fmt.Errorf("cipd does not exist at location: %s", cipdBinary)
	}
	return cipdBinary, nil

}

// Ensure calls cipd on ensureFile with cipd. Timeout in minutes
func Ensure(jirix *jiri.X, ensureFile, projectRoot string, timeout uint) error {
	cipdBinary, err := getCipdPath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
	defer cancel()
	args := []string{"ensure", "-ensure-file", ensureFile, "-root", projectRoot, "-log-level", "warning"}
	jirix.Logger.Debugf("Invoke cipd with %v", args)
	command := exec.CommandContext(ctx, cipdBinary, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	return command.Run()
}
