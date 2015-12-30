package foo

type st struct {
	err error
}

func Foo(s st) {
	println(s.err.Error())
}
