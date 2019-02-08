# Build Jiri

As a project written in golang, building jiri is a straight forward process. Once golang is installed and $GOPATH is set, you can build jiri from source using following command:

```
go get fuchsia.googlesource.com/jiri/cmd/jiri
```

The source of jiri will be cloned into $GOPATH/src/fuchsia.googlesource.com/jiri directory.

If you made any modification to the source and would like to rebuild jiri, you can use following command:

```
go install fuchsia.googlesource.com/jiri/cmd/jiri
```