# interfacer

[![Build Status](https://travis-ci.org/mvdan/interfacer.svg?branch=master)](https://travis-ci.org/mvdan/interfacer)

A linter that suggests interface types. In other words, it warns about
the usage of types that are more specific than necessary.

	go get github.com/mvdan/interfacer/cmd/interfacer

### Usage

```go
func ProcessInput(f *os.File) error {
        b, err := ioutil.ReadAll(f)
        if err != nil {
                return err
        }
        return processBytes(b)
}
```

```sh
$ interfacer ./...
foo.go:10:19: f can be io.Reader
```

### Basic idea

This package relies on `go/types` for the heavy lifting: name
resolution, constant folding and type inference.

It inspects the parameters of your functions to see if they fit an
interface type that is less specific than the current type.

The example above illustrates this point. Overly specific interfaces
also trigger a warning - if `f` were an `io.ReadCloser`, the same
message would appear.

It suggests interface types defined both in `std` and in your packages.
To avoid false positives, it never does any suggestions on functions
that may be implementing an interface method or a named function type.

### Suppressing warnings

If a suggestion is technically correct but doesn't make sense, you can
still suppress the warning by mentioning the type in the function name:

```go
func ProcessFile(f *os.File) error {
	// ...
}
```

### Caveats

* No vendor support on Go 1.5 or earlier. This is because the std
  package `go/build` adds the necessary parts for `go/loader` to support
  vendoring in Go 1.6.
