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

type st struct{}

func (s *st) Basic(c io.Closer) {
	c.Close()
}

func (s *st) BasicWrong(rc io.ReadCloser) {
	rc.Close()
}
