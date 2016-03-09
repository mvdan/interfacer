package foo

type mint int

func (m mint) Close() error {
	return nil
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

type Fooer interface {
	Foo()
}

type FooBarer interface {
	Fooer
	Bar()
}

type holdFooer struct {
	f Fooer
}

type holdFooBarer struct {
	fb FooBarer
}

func Correct(fb FooBarer) {
	_ = holdFooBarer{fb: fb}
}

func CorrectNoKey(fb FooBarer) {
	_ = holdFooBarer{fb}
}

func Wrong(fb FooBarer) {
	_ = holdFooer{f: fb}
}

func WrongNoKey(fb FooBarer) {
	_ = holdFooer{fb}
}

func WrongNoKeyInplace(fb FooBarer) {
	_ = struct {
		f Fooer
	}{fb}
}

func WrongNoKeyMultiple(fb FooBarer) {
	_ = struct {
		f Fooer
		s string
	}{fb, "bar"}
}

type holdFooerNested holdFooer

func WrongNoKeyDeep(fb FooBarer) {
	_ = holdFooerNested{fb}
}

func WrongNoKeyArray(fb FooBarer) {
	_ = [...]Fooer{fb}
}

func WrongNoKeySlice(fb FooBarer) {
	_ = []Fooer{fb}
}

func WrongWalkValue(fb FooBarer) {
	fn := func(f Fooer) Fooer { return f }
	_ = []Fooer{fn(fb)}
}
