package foo

import (
	"io"
)

var Basic = func(c io.Closer) {
	c.Close()
}

var BasicWrong = func(rc io.ReadCloser) {
	rc.Close()
}
