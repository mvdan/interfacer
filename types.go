/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
)

var suggested = [...]string{
	"io.ByteReader",
	"io.ByteScanner",
	"io.ByteWriter",
	"io.Closer",
	"io.ReadCloser",
	"io.ReadSeeker",
	"io.ReadWriteCloser",
	"io.ReadWriteSeeker",
	"io.ReadWriter",
	"io.Reader",
	"io.ReaderAt",
	"io.ReaderFrom",
	"io.RuneReader",
	"io.RuneScanner",
	"io.Seeker",
	"io.WriteCloser",
	"io.WriteSeeker",
	"io.Writer",
	"io.WriterAt",
	"io.WriterTo",
}

type funcDecl struct {
	params  []types.Type
	results []types.Type
}

var parsed map[string]map[string]funcDecl

func typesInit() error {
	fset := token.NewFileSet()
	// Simple program that imports and uses all needed packages
	const typesProgram = `
	package types
	import "io"
	func foo(r io.Reader) {
	}
	`
	f, err := parser.ParseFile(fset, "foo.go", typesProgram, 0)
	if err != nil {
		return err
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
	if err != nil {
		return err
	}
	pos := pkg.Scope().Lookup("foo").Pos()

	parsed = make(map[string]map[string]funcDecl, len(suggested))
	for _, v := range suggested {
		tv, err := types.Eval(fset, pkg, pos, v)
		if err != nil {
			return err
		}
		t := tv.Type
		if !types.IsInterface(t) {
			return fmt.Errorf("%s is not an interface", v)
		}
		named := t.(*types.Named)
		ifname := named.String()
		iface := named.Underlying().(*types.Interface)
		if _, e := parsed[ifname]; e {
			return fmt.Errorf("%s is duplicated", ifname)
		}
		parsed[ifname] = make(map[string]funcDecl, iface.NumMethods())
		for i := 0; i < iface.NumMethods(); i++ {
			f := iface.Method(i)
			fname := f.Name()
			sign := f.Type().(*types.Signature)
			parsed[ifname][fname] = funcDecl{
				params:  typeList(sign.Params()),
				results: typeList(sign.Results()),
			}
		}
	}
	return nil
}

func typeList(t *types.Tuple) []types.Type {
	var l []types.Type
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type())
	}
	return l
}
