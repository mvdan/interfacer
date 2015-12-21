package simple

import (
	"io"
)

type st struct{}

func (s *st) Close() error {
	return nil
}

func (s *st) Basic(c io.Closer) {
	c.Close()
}

func (s *st) BasicWrong(rc io.ReadCloser) {
	rc.Close()
}
