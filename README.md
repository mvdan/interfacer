# interfacer

A code checker that suggests interface types.

	go get github.com/mvdan/interfacer

### Usage

```go
func DoClose(f *os.File) {
	f.Close()
}
```

```sh
$ interfacer ./...
foo.go:10: f can be io.Closer
```
