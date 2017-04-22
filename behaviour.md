# Jiri

[toc]

## Feedback
If jiri does not behave as [intended](#intended-behavior), and if you work at google, please file a bug at [go/file-jiri-bug][file bug], or to request new features use [go/jiri-new-feature][request new feature].
If filing a bug please include output from `jiri [command]`. If you think that jiri did not update project correctly, please also include outputs of `jiri status` and `jiri project` and if possible `jiri update -v`

## Intended Behavior
### update {#intended-jiri-update}
* This command updates all the repos in manifest. It will get the latest manifest and update all projects according to it
* It will always fetch origin in all the repos except when configured using [`jiri project-config`](#intended-project-config)
* It will checkout all new repos into *DETACHED_HEAD* state and will fast-forward all local repos which are on *DETACHED_HEAD* to **JIRI_HEAD**(revision according to manifest)
* If local repo is on a tracked branch, it will rebase it on top of it's tracked branch
* If project is on un-tracked branch it would be left alone and jiri will show warning(can be over-ridden with `-rebase-untracked` flag)
* It will leave all other local branches as it is( can be over-ridden with `-rebase-all` flag
* If a project is deleted from manifest it will complain about it, but won't delete it unless it is run with `-gc` flag
* If a project contains changes, jiri will leave it alone and will not fast-forward or rebase the branches
* Sometimes projects are pinned to particular revesion in manifest, in that case if local project is on a local branch, jiri will update them according to above rules and will not complain about those projects.
    * Please note that this can leave projects on revisions other than **JIRI_HEAD** which can cause build failures. In that case user can run [`jiri status`](#use-jiri-status) which will output all the projects which have changes and/or are not on **JIRI_HEAD**. User can manually checkout **JIRI_HEAD** by running `git checkout JIRI_HEAD` from inside the project.
* If user doesn't want jiri to update a project, he/she can use `jiri project-config`
* This command always updates your jiri tool to latest unless flag `-autoupdate=false` is passed
#### checkout snapshot {#intended-checkout-snapshot}
* snapshot file or a url can be passed to `update` command to checkout snapshot
* If project has changes, it would **not** be checked-out to snapshot version
	* else it would be checked out to *DETACHED_HEAD* and snapshot version
* if `project-config` specifies `ignore` or `noUpdate`, it would be ignored
* Local branches would **not** be rebased

### project-config {#intended-project-config}
* If `ignore` is true, jiri will completely ignore this project, ie **no** *fetch*, *update*, *move*, *clean*, *delete* or *rebase*.
* If `noUpdate` is true, jiri will  **not** *fetch*, *update*, *clean* or *rebase* the project.
* For both `ignore` and `noUpdate`, **JIRI_HEAD** is **not** updated for the project.
* If `noRebase` is true, local branches in project **won't be** *updated* or *rebased*.
* This only works with `update` and `project -clean` commands.

### project -clean {#intended-project-clean}
* Puts projects on **JIRI_HEAD**
* Removes un-tracked files
* if `-clean-all` flag is used, force deletes all the local branches, even **master**

### upload {#intended-project-upload}
* Sets topic(default *User-Branch*) for each upload unless `set-topic=false` is used
* Doesn't rebase the changes before uploading unless `-rebase` is passed
* Uploads multipart change only when `-multipart` is passed

### patch {#intended-patch}
* Can patch multipart and single changes
* If topic is provided patch will try to download whole topic and patch all the affected projects, and will try to create branch derived from topic name
* If topic is not provided default branch would be *change/{id}/{patch-set}*
* It will **not** rebase downloaded patch-set unless `-rebase` flag is provided
* Sets topic(default *User-Branch*) for each upload unless `set-topic=false` is used
* Doesn't rebase the changes before uploading unless `-rebase` is passed
* Uploads multipart change only when `-multipart` is passed

## How Do I
### rebase all my branches
Run  `jiri update -rebase-all`. This will not rebase un-tracked branches.

### rebase my untracked branches
Run `jiri update -rebase-untracked` to rebase your current un-tracked branch. To rebase all un-tracked branches use `jiri update -rebase-all -rebase-untracked`

### test my local manifest changes
`jiri update -local-manifest`

### stop jiri from updating my project
Use `jiri project-config`. [See this](#intended-project-config) for it's intended behavior
Current config can be displayed using command `jiri project-config`
To change a config use
```
jiri project-config [-flag=true|false]
```
where flags are `-ignore`, `no-rebase`, `no-update`

### check if all my projects are on **JIRI_HEAD** {#use-jiri-status}
Run `jiri status ` for that. This command returns all projects which are not on **JIRI_HEAD**, or have un-merged commits, or have un-committed changes

To just get projects which are **not** on **JIRI_HEAD** run
```
jiri status -changes=false -commits=false
```
### run a command inside all my projects
`jiri runp [command]`

### grep across projects
`jiri grep [text]`

### delete branch across projects
Run `jiri branch -d [branch_name]`, this will run `git branch -d [branch_name]` in all the projects. `-D` can also be used to replicate functionality of `git branch -D`

### get projects and branches other than master
`jiri branch`

### update jiri without updating projects
`jiri selfupdate`

### use upload to push CL
[See This][Gerrit Cl Workflow]

### get JIRI_HEAD revision of a project
`git rev-parse JIRI_HEAD` from inside the project

### get current revision of a project
`jiri project [project-name]`

### clean project(s)
Run `jiri project [-clean|clean-all] [project-name]`. See it's [intended behaviour](#intended-project-clean)

### get help
Run `jiri help` to see all the commands and `jiri help [command]` to get help for that command

To give feedback [see this](#feedback)


[Gerrit Cl Workflow]: /README.md#Gerrit-CL-workflow
[file bug]:http://go/file-jiri-bug
[request new feature]: http://go/jiri-new-feature
