package foo

type fooFunc func() error

type st struct {
	foo fooFunc
}

func FooWrong(s *st) {
	s.foo()
}
