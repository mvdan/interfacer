/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"go/importer"
	"go/types"
	"regexp"
)

var pkgs = [...]string{
	"encoding",
	"encoding/binary",
	"flag",
	"fmt",
	"hash",
	"io",
	"net",
	"net/http",
	"sort",
	"sync",
}

type funcSign struct {
	params  []types.Type
	results []types.Type
}

type ifaceSign struct {
	t types.Type

	funcs map[string]funcSign
}

var ifaces map[string]ifaceSign

func typesInit() error {
	ifaces = make(map[string]ifaceSign)
	imp := importer.Default()
	for _, path := range pkgs {
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		grabFromScope(pkg.Scope(), true, path)
	}
	grabFromScope(types.Universe, true, "")
	return nil
}

var exported = regexp.MustCompile(`^[A-Z]`)

func grabFromScope(scope *types.Scope, unexported bool, impPath string) {
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if !unexported && !exported.MatchString(tn.Name()) {
			continue
		}
		t := tn.Type()
		if impPath != "" {
			name = impPath + "." + name
		}
		if _, e := ifaces[name]; e {
			continue
		}
		iface, ok := t.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		if iface.NumMethods() == 0 {
			continue
		}
		ifsign := ifaceSign{
			t:     iface,
			funcs: make(map[string]funcSign, iface.NumMethods()),
		}
		for i := 0; i < iface.NumMethods(); i++ {
			f := iface.Method(i)
			fname := f.Name()
			sign := f.Type().(*types.Signature)
			ifsign.funcs[fname] = funcSign{
				params:  typeList(sign.Params()),
				results: typeList(sign.Results()),
			}
		}
		ifaces[name] = ifsign
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
