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
	"io"
	"log"
	"os"
)

type method struct {
	params []interface{}
}

var parsed map[string]map[string]method

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

func typeList(t *types.Tuple) []interface{} {
	var l []interface{}
	for i := 0; i < t.Len(); i++ {
		v := t.At(i)
		l = append(l, v.Type())
	}
	return l
}

func typeMap(t *types.Tuple) map[string]types.Type {
	m := make(map[string]types.Type, t.Len())
	for i := 0; i < t.Len(); i++ {
		p := t.At(i)
		m[p.Name()] = p.Type()
	}
	return m
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

	parsed = make(map[string]map[string]method, len(suggested))
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
		parsed[ifname] = make(map[string]method, iface.NumMethods())
		for i := 0; i < iface.NumMethods(); i++ {
			f := iface.Method(i)
			fname := f.Name()
			sign := f.Type().(*types.Signature)
			parsed[ifname][fname] = method{
				params: typeList(sign.Params()),
			}
		}
	}
}

func init() {
	typesInit()
}

var toToken = map[string]token.Token{
	"int":   token.INT,
	"int32": token.INT,
	"int64": token.INT,
}

func argEqual(t1 types.Type, a2 interface{}) bool {
	switch x := a2.(type) {
	case string:
		return t1.String() == x
	case token.Token:
		return toToken[t1.String()] == x
	case nil:
		switch t1.(type) {
		case *types.Slice:
			return true
		case *types.Map:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func typesMatch(args1, args2 []interface{}) bool {
	if len(args1) != len(args2) {
		return false
	}
	for i, a1 := range args1 {
		a2 := args2[i]
		t1 := a1.(types.Type)
		if !argEqual(t1, a2) {
			return false
		}
	}
	return true
}

func interfaceMatching(methods map[string]method) string {
	matchesIface := func(decls map[string]method) bool {
		if len(methods) > len(decls) {
			return false
		}
		for n, d := range decls {
			m, e := methods[n]
			if !e {
				return false
			}
			if !typesMatch(d.params, m.params) {
				return false
			}
		}
		return true
	}
	for name, decls := range parsed {
		if matchesIface(decls) {
			return name
		}
	}
	return ""
}

func main() {
	parseFile("stdin.go", os.Stdin, os.Stdout)
}

func parseFile(name string, r io.Reader, w io.Writer) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, r, 0)
	if err != nil {
		log.Fatal(err)
	}

	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("", fset, []*ast.File{f}, nil)
	if err != nil {
		log.Fatal(err)
	}

	v := &Visitor{
		w:      w,
		fset:   fset,
		scopes: []*types.Scope{pkg.Scope()},
	}
	ast.Walk(v, f)
}

type Visitor struct {
	w      io.Writer
	fset   *token.FileSet
	scopes []*types.Scope

	nodes []ast.Node

	params map[string]types.Type
	used   map[string]map[string]method
}

func scopeName(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return scopeName(x.X)
	default:
		return ""
	}
}

func (v *Visitor) getFuncType(fd *ast.FuncDecl) *types.Func {
	name := fd.Name.Name
	scope := v.scopes[len(v.scopes)-1]
	if fd.Recv == nil {
		return scope.Lookup(name).(*types.Func)
	}
	if len(fd.Recv.List) > 1 {
		return nil
	}
	tname := scopeName(fd.Recv.List[0].Type)
	st := scope.Lookup(tname).(*types.TypeName)
	named := st.Type().(*types.Named)
	for i := 0; i < named.NumMethods(); i++ {
		f := named.Method(i)
		if f.Name() == name {
			return f
		}
	}
	return nil
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	switch x := node.(type) {
	case *ast.File:
	case *ast.FuncDecl:
		f := v.getFuncType(x)
		if f == nil {
			return nil
		}
		v.scopes = append(v.scopes, f.Scope())
		sign := f.Type().(*types.Signature)
		v.params = typeMap(sign.Params())
		v.used = make(map[string]map[string]method, 0)
	case *ast.BlockStmt:
	case *ast.ExprStmt:
	case *ast.CallExpr:
		v.onCall(x)
	case nil:
		top := v.nodes[len(v.nodes)-1]
		v.nodes = v.nodes[:len(v.nodes)-1]
		if _, ok := top.(*ast.FuncDecl); !ok {
			return nil
		}
		v.scopes = v.scopes[:len(v.scopes)-1]
		for name, methods := range v.used {
			iface := interfaceMatching(methods)
			if iface == "" {
				continue
			}
			if iface == v.params[name].String() {
				continue
			}
			pos := v.fset.Position(top.Pos())
			fmt.Fprintf(v.w, "%s:%d: %s can be %s\n",
				pos.Filename, pos.Line, name, iface)
		}
		v.params = nil
		v.used = nil
	default:
		return nil
	}
	if node != nil {
		v.nodes = append(v.nodes, node)
	}
	return v
}

func getType(scope *types.Scope, name string) interface{} {
	if scope == nil {
		return nil
	}
	obj := scope.Lookup(name)
	if obj == nil {
		return getType(scope.Parent(), name)
	}
	switch x := obj.(type) {
	case *types.Var:
		return x.Type().String()
	default:
		return nil
	}
}

func (v *Visitor) descType(e ast.Expr) interface{} {
	switch x := e.(type) {
	case *ast.Ident:
		scope := v.scopes[len(v.scopes)-1]
		return getType(scope, x.Name)
	case *ast.BasicLit:
		return x.Kind
	default:
		return nil
	}
}

func (v *Visitor) onCall(c *ast.CallExpr) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	left, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	right := sel.Sel
	vname := left.Name
	fname := right.Name
	m := method{}
	for _, a := range c.Args {
		m.params = append(m.params, v.descType(a))
	}
	if _, e := v.used[vname]; !e {
		v.used[vname] = make(map[string]method)
	}
	v.used[vname][fname] = m
}
