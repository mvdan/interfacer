package foo

type Closer interface {
	Close() error
}

type Reader interface {
	Read(p []byte) (n int, err error)
}

type ReadCloser interface {
	Reader
	Closer
}

type st struct{}

func (s st) Read(p []byte) (int, error) {
	return 0, nil
}
func (s st) Close() error {
	return nil
}
func (s st) Other() {}

func FooCloser(c Closer) {
	c.Close()
}

func FooSt(s st) {
	s.Other()
}

func Bar(s st) {
	s.Close()
	FooSt(s)
}

func BarWrong(s st) {
	s.Close()
	FooCloser(s)
}

func extra(n int, cs ...Closer) {}

func ArgExtraWrong(s1 st) {
	var s2 st
	s1.Close()
	s2.Close()
	extra(3, s1, s2)
}

func AssignedStruct(s st) {
	s.Close()
	var s2 st
	s2 = s
	_ = s2
}

func AssignedIface(s st) {
	s.Close()
	var c Closer
	c = s
	_ = c
}

func AssignedIfaceDiff(s st) {
	s.Close()
	var r Reader
	r = s
	_ = r
}

func doRead(r Reader) {
	b := make([]byte, 10)
	r.Read(b)
}

func ArgIfaceDiff(s st) {
	s.Close()
	doRead(s)
}
