# interfacer

A code checker that suggests interface types. In other words, it warns
about the usage of types that are more specific than necessary.

	go get github.com/mvdan/interfacer/cmd/interfacer

### Usage

```go
func ProcessInput(f *os.File) error {
        b := make([]byte, 64)
        if _, err := f.Read(b); err != nil {
                return err
        }
        // process b
        return nil
}
```

```sh
$ interfacer ./...
foo.go:10: f can be io.Reader
```
