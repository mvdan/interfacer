package foo

import (
	"io"
)

func Empty() {
}

func Basic(c io.Closer) {
	c.Close()
}

func BasicWrong(rc io.ReadCloser) {
	rc.Close()
}

func StructWrong(s st) {
	s.Close()
}
