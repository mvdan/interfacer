// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"bytes"
	"io"
	"regexp"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

//go:generate go run generate/std/main.go generate/std/pkgs.go

type funcSign struct {
	params  []types.Type
	results []types.Type
}

func (fs funcSign) String() string {
	var b bytes.Buffer
	io.WriteString(&b, "[")
	for _, t := range fs.params {
		io.WriteString(&b, t.String())
		io.WriteString(&b, ",")
	}
	io.WriteString(&b, "] [")
	for _, t := range fs.results {
		io.WriteString(&b, t.String())
		io.WriteString(&b, ",")
	}
	io.WriteString(&b, "]")
	return b.String()
}

type ifaceSign struct {
	name string
	t    *types.Interface

	funcs map[string]funcSign
}

type cache struct {
	loader.Config

	// key is importPath.typeName
	pkgIfaces map[string][]ifaceSign

	std map[string]bool

	curPaths []string

	funcs map[string]funcSign
}

func typesInit() error {
	c = &cache{
		pkgIfaces: make(map[string][]ifaceSign),
		funcs:     make(map[string]funcSign),
		std:       make(map[string]bool),
	}
	c.AllowErrors = true
	c.TypeChecker.Error = func(e error) {}
	c.TypeCheckFuncBodies = func(path string) bool {
		return !c.std[path]
	}
	c.TypeChecker.DisableUnusedImportCheck = true
	// TODO: once loader is ported to go/types, cache imported std
	// packages across tests
	for _, p := range pkgs {
		if p.path == "" {
			continue
		}
		c.std[p.path] = true
		if len(p.names) < 1 {
			continue
		}
		c.Import(p.path)
	}
	return nil
}

func typesGet(prog *loader.Program) {
	done := make(map[string]bool)
	for _, p := range pkgs {
		path := p.path
		done[path] = true
		scope := types.Universe
		if path != "" {
			pkg := prog.Package(path)
			if pkg == nil {
				continue
			}
			scope = pkg.Pkg.Scope()
		}
		c.grabNames(scope, path, p.names)
	}
	for _, pkg := range prog.InitialPackages() {
		path := pkg.Pkg.Path()
		if done[path] {
			continue
		}
		c.grabExported(pkg.Pkg.Scope(), path)
	}
}

func (c *cache) grabNames(scope *types.Scope, path string, names []string) {
	for _, name := range names {
		tn := scope.Lookup(name).(*types.TypeName)
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			c.addInterface(path, name, x)
		case *types.Signature:
			c.addFunc(x)
		}
	}
}

func (c *cache) addInterface(path, name string, iface *types.Interface) {
	ifsign := ifaceSign{
		name:  name,
		t:     iface,
		funcs: make(map[string]funcSign, iface.NumMethods()),
	}
	for i := 0; i < iface.NumMethods(); i++ {
		f := iface.Method(i)
		sign := f.Type().(*types.Signature)
		ifsign.funcs[f.Name()] = c.addFunc(sign)
	}
	c.pkgIfaces[path] = append(c.pkgIfaces[path], ifsign)
}

func (c *cache) addFunc(sign *types.Signature) funcSign {
	fsign := funcSign{
		params:  typeList(sign.Params()),
		results: typeList(sign.Results()),
	}
	s := fsign.String()
	if _, e := c.funcs[s]; !e {
		c.funcs[s] = fsign
	}
	return fsign
}

var exported = regexp.MustCompile(`^[A-Z]`)

func (c *cache) grabExported(scope *types.Scope, path string) {
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if !exported.MatchString(tn.Name()) {
			continue
		}
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			if x.NumMethods() == 0 {
				continue
			}
			c.addInterface(path, name, x)
		case *types.Signature:
			c.addFunc(x)
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
