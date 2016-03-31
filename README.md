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

### Algorithm

This package relies on `go/types` for the heavy lifting: name
resolution, constant folding and type inference.

Once all the types are clear, it inspects every declared function and
sees if any of the arguments could be better typed. It uses a string
representation of functions and interfaces to find exact matches.

To illustrate this point, have a look at [std.go](std.go). This
[generated](generate/std/) file contains all the interfaces and
functions that were found in the standard library which can be of use to
us. Interfaces which contain unexported methods must be discarded, for
example.

The `ifaces` map is self-explanatory. The `funcs` map can be deceiving -
it contains all the function types that may be purposedly implemented.
These come from interfaces, such as `Read()`, and from function types,
such as `WalkFunc`.

When checking a series of packages, it builds on top of these two maps
with the types it finds along the way.

Next it uses `go/ast` to walk the source code. For every declared
function, it first checks if its signature matches any recorded function
type. If it does, it is skipped.

Then it keeps track of the function's parameters and how they are used. In
particular, we are interested in:

* Whether or not it can be an interface type
* What methods are called on it
* What types it is assigned to and passed as

The first one is pretty straightforward - if a param `p` is used like
`p.field`, `p + 3` or `p[2]`, it definitely cannot be an interface.

The second one builds a set of function signatures for all method calls.
This is later used to find an interface type that exactly matches this
method usage. Since we represent method sets as strings, this is as
simple as indexing the interfaces map.

As for the types that it is assigned to and passed as, we first look at
whether each of the types is an interface - if it isn't, the argument
cannot be an interface. Otherwise, we assume that all the funcs of the
interface may be called on the argument, as if they were all called
directly.

If the found interface type doesn't match the current parameter type, we
found a suggestion to print out.

### Caveats

* No vendor support on Go 1.5 or earlier. This is because the std
  package `go/build` adds the necessary parts for `go/loader` to support
  vendoring in Go 1.6.
