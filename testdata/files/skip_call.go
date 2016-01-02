package foo

type st struct {
	err error
}

func Foo(s st) {
	println(s.err.Error())
}

func NoArgs(s int) {
	println()
}

type mint int

func (m mint) Close() error {
	return nil
}

func ConvertBasic(m mint) {
	m.Close()
	_ = int64(m)
}

type mstr string

func (m mstr) Close() error {
	return nil
}

func ConvertSlice(m mstr) {
	m.Close()
	_ = []byte(m)
}

type myFunc func() error

func ConvertNoArg(f myFunc) {
	_ = f()
}
