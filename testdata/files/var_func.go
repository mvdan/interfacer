package foo

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

var Basic = func(c Closer) {
	c.Close()
}

var BasicWrong = func(rc ReadCloser) {
	rc.Close()
}

type st struct{}

func (s st) Close() error {
	return nil
}

type MyFunc func(s st, err error) bool

var MyFuncImpl = func(s st, err error) bool {
	s.Close()
	return false
}

var MyFuncWrong = func(s st, err error) {
	s.Close()
}
