package foo

import (
	"net/http"
)

func serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{})
}

func serveHTTPWrong(w http.ResponseWriter) {
	w.Write([]byte{})
}
