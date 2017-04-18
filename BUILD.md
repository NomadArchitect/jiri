# Building  Jiri

## Get source

### Using jiri prebuilt
This method only works with linux and darwin `x86_64` systems
The bootstrap procedure requires that you have Go 1.6 or newer and Git installed and on your `PATH`. Below command will create checkout in new folder called `funchsia`
```
curl -s
https://raw.githubusercontent.com/fuchsia-mirror/jiri/master/scripts/bootstrap_jiri | bash -s fuchsia
cd fuchsia
export PATH=`pwd`/.jiri_root/bin:$PATH
jiri import jiri https://fuchsia.googlesource.com/manifest
jiri update
```
### Manually
Create a root folder called `fuchsia`, then use git to manually clone each of the projects mentioned in this [manifest][jiri manifest], put them in correct paths and checkout required revisions. `HEAD` should be on `origin/master` where no revision is mentioned in manifest

## Build
Set GOPATH to `fuchsia/go`, cd into `fuchsia/go/src/fuchsia.googlesource.com/jiri` and run
```
./script/build.sh
```

The above command should build jiri and put it into your current folder

## Known Issues

If build complains about undefined `http_parser_*` functions, please remove `http_parser` from your system

[jiri manifest]: https://fuchsia.googlesource.com/manifest/+/refs/heads/master/jiri "jiri manifest"
