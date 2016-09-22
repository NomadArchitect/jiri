# Testing manifest changes locally

Jiri has behavior that makes testing manifest changes locally annoyingly
difficult.  Here's a workflow that I've successfully used.

## Setup

The technique here is to create a local manifest repo that will contain your changes.

```sh
13:27:07 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing ●  ls -la
total 0
drwxr-xr-x   2 kitten  staff   68 Sep 22 13:27 .
drwx------+ 11 kitten  staff  374 Sep 21 17:50 ..
13:27:08 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing ●  git clone https://fuchsia.googlesource.com/manifest
Cloning into 'manifest'...
remote: Total 379 (delta 58), reused 379 (delta 58)
Receiving objects: 100% (379/379), 154.93 KiB | 0 bytes/s, done.
Resolving deltas: 100% (58/58), done.
Checking connectivity... done.
13:27:57 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing ●  cd manifest
13:28:01 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  vi minimal 
# Change manifest remote to current working directory (in this case: /Users/kitten/Documents/jiri-testing/manifest)
13:29:03 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git commit -a
[master 28f635b] [DO NOT SUBMIT] set manifest remote to local repo
 1 file changed, 1 insertion(+), 1 deletion(-)
13:29:23 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  cd ..
13:29:36 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing ●  curl -s https://raw.githubusercontent.com/fuchsia-mirror/jiri/master/scripts/bootstrap_jiri | bash -s fuchsia
Please add /Users/kitten/Documents/jiri-testing/fuchsia/.jiri_root/scripts to your PATH
13:29:57 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing ●  cd fuchsia/
13:30:00 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  jiri import minimal ~/Documents/jiri-testing/manifest
13:30:27 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  jiri update
[13:30:31.74] >> move project "manifest" located in "/var/folders/p1/8_1psss92md74pj65fvbq4l80000gn/T/jiri-load565110349/manifest_eb6fb048998e5e67" to "/Users/kitten/Documents/jiri-testing/fuchsia/manifest" and advance it to "HEAD"
[13:30:31.94] >> OK
[13:30:31.94] >> create project "jiri" in "/Users/kitten/Documents/jiri-testing/fuchsia/go/src/fuchsia.googlesource.com/jiri" and advance it to "68661f35"
[13:30:33.92] >> OK
[13:30:33.92] >> build tools: jiri
[13:30:43.43] >> OK
[13:30:43.43] >> install tool "jiri"
[13:30:43.43] >> OK
13:30:43 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  ls -l
total 0
drwxr-xr-x   3 kitten  staff  102 Sep 22 13:30 devtools
drwxr-xr-x   3 kitten  staff  102 Sep 22 13:30 go
drwxr-xr-x  20 kitten  staff  680 Sep 22 13:30 manifest
13:30:50 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  
```

## Making and testing a change

Make a change in the minimal repo, commit it, then test it with `jiri update`.

```sh
13:35:37 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  vi minimal 
13:35:54 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git commit -a
[master 6866dd2] Remove jiri repo/tool from minimal manifest
 1 file changed, 10 deletions(-)
13:36:11 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  cd ../fuchsia/
13:36:23 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  jiri update -gc
[13:36:28.46] >> delete project "jiri" from "/Users/kitten/Documents/jiri-testing/fuchsia/go/src/fuchsia.googlesource.com/jiri"
[13:36:28.50] >> OK
[13:36:28.50] >> advance project "manifest" located in "/Users/kitten/Documents/jiri-testing/fuchsia/manifest" to "HEAD"
[13:36:28.70] >> OK
[13:36:28.71] >> build tools: 
[13:36:28.86] >> OK
13:36:29 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/fuchsia ●  
```

## Uploading the tested change

You can also upload the change directly from the test repo, with a little git+gerrit wrangling.

```sh
13:44:50 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git log --graph --abbrev-commit --pretty=oneline --decorate --all -n5
* 6866dd2 (HEAD -> master) Remove jiri repo/tool from minimal manifest
* 28f635b [DO NOT SUBMIT] set manifest remote to local repo
* 78cb3f2 (origin/master, origin/HEAD) Throwing in the towel!
* 34250cf Additional Dart/Flutter dependencies.
* 8eb2eb3 Import toolchain manifest into 'experience' manifest
13:44:54 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git checkout origin/master
Note: checking out 'origin/master'.

You are in 'detached HEAD' state. You can look around, make experimental
changes and commit them, and you can discard any commits you make in this
state without impacting any branches by performing another checkout.

If you want to create a new branch to retain commits you create, you may
do so (now or later) by using -b with the checkout command again. Example:

  git checkout -b <new-branch-name>

HEAD is now at 78cb3f2... Throwing in the towel!
13:45:10 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git cherry-pick master
[detached HEAD 2b00161] Remove jiri repo/tool from minimal manifest
 Date: Thu Sep 22 13:35:56 2016 -0700
 1 file changed, 10 deletions(-)
13:45:19 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git log --graph --abbrev-commit --pretty=oneline --decorate --all -n5
* 2b00161 (HEAD) Remove jiri repo/tool from minimal manifest
| * 6866dd2 (master) Remove jiri repo/tool from minimal manifest
| * 28f635b [DO NOT SUBMIT] set manifest remote to local repo
|/  
* 78cb3f2 (origin/master, origin/HEAD) Throwing in the towel!
* 34250cf Additional Dart/Flutter dependencies.
13:45:56 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git remote add gerrit https://fuchsia.googlesource.com/manifest
13:46:21 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://gerrit-review.googlesource.com/tools/hooks/commit-msg ; chmod +x `git rev-parse --git-dir`/hooks/commit-msg
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  4697  100  4697    0     0  21675      0 --:--:-- --:--:-- --:--:-- 21745
13:46:28 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git commit --amend
[detached HEAD af9c165] Remove jiri repo/tool from minimal manifest
 Date: Thu Sep 22 13:35:56 2016 -0700
 1 file changed, 10 deletions(-)
13:46:35 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  git push origin HEAD:refs/for/master
Counting objects: 3, done.
Delta compression using up to 8 threads.
Compressing objects: 100% (3/3), done.
Writing objects: 100% (3/3), 380 bytes | 0 bytes/s, done.
Total 3 (delta 2), reused 0 (delta 0)
remote: Resolving deltas: 100% (2/2)
remote: Processing changes: new: 1, done    
remote: 
remote: New Changes:
remote:   https://fuchsia-review.googlesource.com/10737 Remove jiri repo/tool from minimal manifest
remote: 
To https://fuchsia.googlesource.com/manifest
 * [new branch]      HEAD -> refs/for/master
13:46:44 kitten@Kits-MacBook-Pro ~/Documents/jiri-testing/manifest ●  
```
