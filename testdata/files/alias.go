package foo

import (
	"net/http"
)

type MyCloser interface {
	Close() error
}

func Foo(c MyCloser) {
	c.Close()
}

func WeirdFalsePositive(w http.ResponseWriter) {
	w.Header().Set("a", "b")
	w.WriteHeader(3)
	w.Write([]byte{})
}
