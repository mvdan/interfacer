// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"go/importer"
	"go/types"
	"regexp"
)

//go:generate go run generate/std/main.go generate/std/pkgs.go
//go:generate gofmt -w std.go

type funcSign struct {
	params  []types.Type
	results []types.Type
}

type ifaceSign struct {
	name string
	t    *types.Interface

	funcs map[string]funcSign
}

type cache struct {
	done map[string]struct{}

	// key is importPath.typeName
	// TODO: do something about duplicates, especially to behave
	// deterministically if two keys map to equal ifaceSigns.
	stdIfaces map[string][]ifaceSign

	pkgIfaces map[string][]ifaceSign

	curPaths []string

	// TODO: avoid duplicates
	funcs []funcSign
}

func typesInit() error {
	c = &cache{
		done:      make(map[string]struct{}),
		stdIfaces: make(map[string][]ifaceSign),
		pkgIfaces: make(map[string][]ifaceSign),
	}
	imp := importer.Default()
	for path, names := range pkgs {
		c.done[path] = struct{}{}
		if len(names) == 0 {
			continue
		}
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		c.grabNames(pkg.Scope(), path, names)
	}
	c.grabNames(types.Universe, "", []string{"error"})
	return nil
}

func (c *cache) grabNames(scope *types.Scope, path string, names []string) {
	for _, name := range names {
		tn := scope.Lookup(name).(*types.TypeName)
		switch x := tn.Type().Underlying().(type) {
		case *types.Interface:
			c.addInterface(c.stdIfaces, path, name, x)
		case *types.Signature:
			c.addFunc(x)
		}
	}
}

func (c *cache) addInterface(m map[string][]ifaceSign, path, name string, iface *types.Interface) {
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
	m[path] = append(m[path], ifsign)
}

func (c *cache) addFunc(sign *types.Signature) funcSign {
	fsign := funcSign{
		params:  typeList(sign.Params()),
		results: typeList(sign.Results()),
	}
	c.funcs = append(c.funcs, fsign)
	return fsign
}

var exported = regexp.MustCompile(`^[A-Z]`)

func (c *cache) grabExported(scope *types.Scope, path string) {
	c.done[path] = struct{}{}
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
			c.addInterface(c.pkgIfaces, path, name, x)
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
