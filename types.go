/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"log"
)

type funcDecl struct {
	params []types.Type
}

var parsed map[string]map[string]funcDecl

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

func typesInit() {
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
		log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
	if err != nil {
		log.Fatal(err)
	}
	pos := pkg.Scope().Lookup("foo").Pos()

	parsed = make(map[string]map[string]funcDecl, len(suggested))
	for _, v := range suggested {
		tv, err := types.Eval(fset, pkg, pos, v)
		if err != nil {
			log.Fatal(err)
		}
		t := tv.Type
		if !types.IsInterface(t) {
			log.Fatalf("%s is not an interface", v)
		}
		named := t.(*types.Named)
		ifname := named.String()
		iface := named.Underlying().(*types.Interface)
		if _, e := parsed[ifname]; e {
			log.Fatalf("%s is duplicated", ifname)
		}
		parsed[ifname] = make(map[string]funcDecl, iface.NumMethods())
		for i := 0; i < iface.NumMethods(); i++ {
			f := iface.Method(i)
			fname := f.Name()
			sign := f.Type().(*types.Signature)
			parsed[ifname][fname] = funcDecl{
				params: typeList(sign.Params()),
			}
		}
	}
}

func typeList(t *types.Tuple) []types.Type {
	var l []types.Type
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type())
	}
	return l
}
