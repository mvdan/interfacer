package foo

type st struct {
	field int
}

func (s *st) Close() error {
	return nil
}

func Foo(s *st) {
	s.Close()
	s.field = 3
}

func FooWrong(s *st) {
	s.Close()
}

type st2 struct {
	st1 *st
}

func (s *st2) Close() error {
	return nil
}

func Foo2(s *st2) {
	s.Close()
	s.st1.field = 3
}

func Foo2Wrong(s *st2) {
	s.Close()
}

func FooPassed(s *st) {
	s.Close()
	s2 := s
	s2.field = 2
}

func FooPassedWrong(s *st) {
	s.Close()
	s2 := s
	s2.Close()
}
