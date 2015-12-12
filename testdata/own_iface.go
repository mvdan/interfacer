package foo

type Barer interface {
	Bar() int
}

type st struct {}

func (s *st) Bar() int {
	return 2
}

func Foo(s *st) {
	_ = s.Bar()
}
