package foo

type mint int

func (m mint) Close() error {
	return nil
}

type st struct {
	m mint
	s string
}

func StructVal(m mint) {
	m.Close()
	_ = st{
		m: m,
	}
}

func MapKey(m mint) {
	m.Close()
	_ = map[mint]string{
		m: "foo",
	}
}

func MapValue(m mint) {
	m.Close()
	_ = map[string]mint{
		"foo": m,
	}
}
