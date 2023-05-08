#!/usr/bin/env bash
# Copyright 2023 The Fuchsia Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Script to configure a git repository with recommended defaults.
# Add --global to make these changes globally.

set -euf -o pipefail

# Configure Gerrit's Change-Id message footer.
f=`git rev-parse --git-dir`/hooks/commit-msg ; mkdir -p $(dirname $f) ; curl -Lo $f https://gerrit-review.googlesource.com/tools/hooks/commit-msg ; chmod +x $f

set -v
# https://git-scm.com/docs/git-config#Documentation/git-config.txt-alias
# Uploads this change to gerrit.
git config alias.gerrit "push origin HEAD:refs/for/main"
# Uploads this change to gerrit and triggers a try job.
git config alias.gerritcq "push origin HEAD:refs/for/main%l=Commit-Queue+1"

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-branchautoSetupMerge
# Tells "git branch", "git switch" and "git checkout" to set up new branches so
# that "git pull" will appropriately merge from the starting point branch.
git config branch.autoSetupMerge always

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-branchautoSetupRebase
# When a new branch is created with git branch, git switch or git checkout that
# tracks another branch, this variable tells Git to set up pull to rebase
# instead of merge.
git config branch.autoSetupRebase always

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-fetchprune
# "git fetch" will automatically behave as if the --prune option was given on
# the command line.
git config fetch.prune true

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-initdefaultBranch
# The Fuchsia project uses "main" instead of "master".
git config init.defaultBranch main

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-pullrebase
# Rebase branches on top of the fetched branch, instead of merging the default
# branch from the default remote when "git pull" is run.
git config pull.rebase true

# https://git-scm.com/docs/git-config#Documentation/git-config.txt-rebaseautoStash
# Automatically create a temporary stash entry before the operation begins, and
# apply it after the operation ends.
git config rebase.autoStash true
