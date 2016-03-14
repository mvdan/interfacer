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

var BasicWrong = func(rc ReadCloser) { // WARN rc can be Closer
	rc.Close()
}

type st struct{}

func (s st) Close() error {
	return nil
}

type MyFunc func(s st) bool

var MyFuncImpl = func(s st) bool {
	s.Close()
	return false
}

var MyFuncWrong = func(s st) { // WARN s can be io.Closer
	s.Close()
}
