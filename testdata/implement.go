package foo

import (
	"net/http"
	"os"
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{})
}

func serveHTTPWrong(w http.ResponseWriter) {
	w.Write([]byte{})
}

type MyFunc func(f *os.File, err error) bool

func MyFuncImpl(f *os.File, err error) bool {
	f.Close()
	return false
}

func MyFuncWrong(f *os.File, err error) {
	f.Close()
}
