package foo

type mint int

func (m mint) Close() error {
	return nil
}

type mint2 mint

func ConvertStruct(m mint) {
	m.Close()
	_ = mint2(m)
}

func ImplicitComparisonConvert(m mint) {
	if m == 0 {
		m.Close()
	}
}

// complex type lit
