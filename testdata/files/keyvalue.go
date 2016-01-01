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
