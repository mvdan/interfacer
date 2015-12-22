package foo

type st struct{}

func (s *st) String() string {
	return ""
}

func (s *st) Error() string {
	return ""
}

func Stringer(s st) {
	s.String()
}

func Error(s st) {
	s.Error()
}
