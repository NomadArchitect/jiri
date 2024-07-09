// Copyright 2016 The Fuchsia Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"go.fuchsia.dev/jiri"
	"go.fuchsia.dev/jiri/analytics_util"
	"go.fuchsia.dev/jiri/cmdline"
)

var cmdInit = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runInit),
	Name:   "init",
	Short:  "Create a new jiri root",
	Long: `
The "init" command creates new jiri "root" - basically a [root]/.jiri_root
directory and template files.

Running "init" in existing jiri [root] is safe.
`,
	ArgsName: "[directory]",
	ArgsLong: `
If you provide a directory, the command is run inside it. If this directory
does not exists, it will be created.
`,
}

var (
	cacheFlag                       string
	dissociateFlag                  bool
	sharedFlag                      bool
	showAnalyticsDataFlag           bool
	analyticsOptFlag                string
	rewriteSsoToHttpsFlag           string
	ssoCookieFlag                   string
	keepGitHooks                    string
	enableLockfileFlag              string
	lockfileNameFlag                string
	prebuiltJSON                    string
	enableSubmodules                string
	forceDisableSubmodulesInfraOnly string
	optionalAttrs                   string
	partialFlag                     bool
	partialSkipFlag                 arrayFlag
	offloadPackfilesFlag            bool
	cipdParanoidFlag                string
	cipdMaxThreads                  int
	excludeDirsFlag                 arrayFlag
)

const (
	optionalAttrsNotSet = "[ATTRIBUTES_NOT_SET]"
)

func init() {
	cmdInit.Flags.StringVar(&cacheFlag, "cache", "", "Jiri cache directory.")
	cmdInit.Flags.BoolVar(&sharedFlag, "shared", false, "[DEPRECATED] All caches are shared.")
	cmdInit.Flags.BoolVar(&dissociateFlag, "dissociate", false, "Dissociate the git cache after a clone or fetch.")
	cmdInit.Flags.BoolVar(&showAnalyticsDataFlag, "show-analytics-data", false, "Show analytics data that jiri collect when you opt-in and exits.")
	cmdInit.Flags.StringVar(&analyticsOptFlag, "analytics-opt", "", "Opt in/out of analytics collection. Takes true/false")
	cmdInit.Flags.StringVar(&rewriteSsoToHttpsFlag, "rewrite-sso-to-https", "", "Rewrites sso fetches, clones, etc to https. Takes true/false.")
	cmdInit.Flags.StringVar(&ssoCookieFlag, "sso-cookie-path", "", "Path to master SSO cookie file.")
	cmdInit.Flags.StringVar(&keepGitHooks, "keep-git-hooks", "", "Whether to keep current git hooks in '.git/hooks' when doing 'jiri update'. Takes true/false.")
	cmdInit.Flags.StringVar(&enableLockfileFlag, "enable-lockfile", "", "Enable lockfile enforcement")
	cmdInit.Flags.StringVar(&lockfileNameFlag, "lockfile-name", "", "Set up filename of lockfile")
	cmdInit.Flags.StringVar(&prebuiltJSON, "prebuilt-json", "", "Set up filename for prebuilt json file")
	cmdInit.Flags.StringVar(&enableSubmodules, "enable-submodules", "", "Enable submodules structure")
	// This flag will be used to forcibly roll out submodules to users while still allowing infra to opt out.
	cmdInit.Flags.StringVar(&forceDisableSubmodulesInfraOnly, "force-disable-submodules-infra-only", "", "Force disable submodules.")
	// Empty string is not used as default value for optionalAttrs as we
	// use empty string to clear existing saved attributes.
	cmdInit.Flags.StringVar(&optionalAttrs, "fetch-optional", optionalAttrsNotSet, "Set up attributes of optional projects and packages that should be fetched by jiri.")
	cmdInit.Flags.BoolVar(&partialFlag, "partial", false, "Whether to use a partial checkout.")
	cmdInit.Flags.Var(&partialSkipFlag, "skip-partial", "Skip using partial checkouts for these remotes.")
	cmdInit.Flags.BoolVar(&offloadPackfilesFlag, "offload-packfiles", true, "Whether to use a CDN for packfiles if available.")
	cmdInit.Flags.StringVar(&cipdParanoidFlag, "cipd-paranoid-mode", "", "Whether to use paranoid mode in cipd.")
	// Default (0) causes CIPD to use as many threads as there are CPUs.
	cmdInit.Flags.IntVar(&cipdMaxThreads, "cipd-max-threads", 0, "Number of threads to use for unpacking CIPD packages. If zero, uses all CPUs.")
	cmdInit.Flags.Var(&excludeDirsFlag, "exclude-dirs", "Directories to skip when searching for local projects (Default: out).")
}

func runInit(env *cmdline.Env, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("wrong number of arguments")
	}

	if showAnalyticsDataFlag {
		fmt.Printf("%s\n", analytics_util.CollectedData)
		return nil
	}

	var dir string
	var err error
	if len(args) == 1 {
		dir, err = filepath.Abs(args[0])
		if err != nil {
			return err
		}
		if _, err := os.Stat(dir); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			if err := os.Mkdir(dir, 0755); err != nil {
				return err
			}
		}
	} else {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	d := filepath.Join(dir, jiri.RootMetaDir)
	if _, err := os.Stat(d); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(d, 0755); err != nil {
			return err
		}
	}

	if cacheFlag != "" {
		cache, err := filepath.Abs(cacheFlag)
		if err != nil {
			return err
		}
		if _, err := os.Stat(cache); os.IsNotExist(err) {
			if err := os.MkdirAll(cache, 0755); err != nil {
				return err
			}
		}
	}

	config := &jiri.Config{}
	configPath := filepath.Join(d, jiri.ConfigFile)
	if _, err := os.Stat(configPath); err == nil {
		config, err = jiri.ConfigFromFile(configPath)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if cacheFlag != "" {
		config.CachePath = cacheFlag
	}

	if dissociateFlag {
		config.Dissociate = true
	}

	if keepGitHooks != "" {
		if val, err := strconv.ParseBool(keepGitHooks); err != nil {
			return fmt.Errorf("'keep-git-hooks' flag should be true or false")
		} else {
			config.KeepGitHooks = val
		}
	}

	if gitConfigSubm, err := jiri.GitGetConfig("jiri.enableSubmodules"); err == nil {
		if _, err := strconv.ParseBool(gitConfigSubm); err != nil {
			return fmt.Errorf("'jiri.enableSubmodules' from git config should be true or false")
		}
		config.EnableSubmodules = gitConfigSubm
	}

	if enableSubmodules != "" {
		if _, err := strconv.ParseBool(enableSubmodules); err != nil {
			return fmt.Errorf("'enable-submodules' flag should be true or false")
		} else {
			config.EnableSubmodules = enableSubmodules
		}
	}

	// Override EnableSubmodules values with False if ForceDisableSubmodulesInfraOnly flag is True
	if forceDisableSubmodulesInfraOnly != "" {
		if val, err := strconv.ParseBool(forceDisableSubmodulesInfraOnly); err != nil {
			return fmt.Errorf("'force-disable-submodules-infra-only' flag should be true or false")
		} else {
			if val {
				config.ForceDisableSubmodulesInfraOnly = "true"
			}
		}
	}

	if rewriteSsoToHttpsFlag != "" {
		if val, err := strconv.ParseBool(rewriteSsoToHttpsFlag); err != nil {
			return fmt.Errorf("'rewrite-sso-to-https' flag should be true or false")
		} else {
			config.RewriteSsoToHttps = val
		}
	}

	if optionalAttrs != optionalAttrsNotSet {
		config.FetchingAttrs = optionalAttrs
	}

	if partialFlag {
		config.Partial = partialFlag
	}

	for _, r := range partialSkipFlag {
		config.PartialSkip = append(config.PartialSkip, r)
	}

	if offloadPackfilesFlag {
		config.OffloadPackfiles = offloadPackfilesFlag
	}

	if ssoCookieFlag != "" {
		config.SsoCookiePath = ssoCookieFlag
	}

	if lockfileNameFlag != "" {
		config.LockfileName = lockfileNameFlag
	}

	if prebuiltJSON != "" {
		config.PrebuiltJSON = prebuiltJSON
	}

	if enableLockfileFlag != "" {
		if _, err := strconv.ParseBool(enableLockfileFlag); err != nil {
			return fmt.Errorf("'enable-lockfile' flag should be true or false")
		}
		config.LockfileEnabled = enableLockfileFlag
	}

	if cipdParanoidFlag != "" {
		if _, err := strconv.ParseBool(cipdParanoidFlag); err != nil {
			return fmt.Errorf("'cipd-paranoid-mode' flag should be true or false")
		}
		config.CipdParanoidMode = cipdParanoidFlag
	}

	config.CipdMaxThreads = cipdMaxThreads

	if analyticsOptFlag != "" {
		if val, err := strconv.ParseBool(analyticsOptFlag); err != nil {
			return fmt.Errorf("'analytics-opt' flag should be true or false")
		} else {
			if val {
				config.AnalyticsOptIn = "yes"
				config.AnalyticsVersion = analytics_util.Version

				bytes := make([]byte, 16)
				io.ReadFull(rand.Reader, bytes)
				if err != nil {
					return err
				}
				bytes[6] = (bytes[6] & 0x0f) | 0x40
				bytes[8] = (bytes[8] & 0x3f) | 0x80

				config.AnalyticsUserId = fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
			} else {
				config.AnalyticsOptIn = "no"
				config.AnalyticsVersion = ""
				config.AnalyticsUserId = ""
			}
		}
	}

	if len(excludeDirsFlag) != 0 {
		config.ExcludeDirs = []string{}
	}

	for _, r := range excludeDirsFlag {
		config.ExcludeDirs = append(config.ExcludeDirs, r)
	}

	if err := config.Write(configPath); err != nil {
		return err
	}
	// TODO(phosek): also create an empty manifest

	return nil
}
