package foo

func Empty() {
}

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

func Basic(c Closer) {
	c.Close()
}

func BasicWrong(rc ReadCloser) {
	rc.Close()
}

type st struct{}

func (s *st) Basic(c Closer) {
	c.Close()
}

func (s *st) BasicWrong(rc ReadCloser) {
	rc.Close()
}
