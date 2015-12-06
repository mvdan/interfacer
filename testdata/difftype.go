package foo

type st struct{}

func (s *st) Read(n int) {}
func (s *st) Close()     {}

func FooArgs(s st) {
	s.Read(3)
}

func FooArgs2(s st) {
	s.Close()
	s.Read(3)
}
