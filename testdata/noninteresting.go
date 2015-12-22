package foo

type EmptyIface interface{}

type UnexpMethodIface interface{
	Foo() error
	bar() int
}
