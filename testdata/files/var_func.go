package foo

import (
	"io"
	"net/http"
)

var Basic = func(c io.Closer) {
	c.Close()
}

var BasicWrong = func(rc io.ReadCloser) {
	rc.Close()
}

var serveHTTP = func(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte{})
}
