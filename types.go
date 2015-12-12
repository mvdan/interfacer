/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"go/importer"
	"go/types"
)

var pkgs = [...]string{
	"encoding",
	"encoding/binary",
	"flag",
	"fmt",
	"hash",
	"io",
	"net",
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

var parsed map[string]ifaceSign

func typesInit() error {
	parsed = make(map[string]ifaceSign)
	imp := importer.Default()
	for _, path := range pkgs {
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		scope := pkg.Scope()
		for _, name := range scope.Names() {
			tn, ok := scope.Lookup(name).(*types.TypeName)
			if !ok {
				continue
			}
			t := tn.Type()
			iface, ok := t.Underlying().(*types.Interface)
			if !ok {
				continue
			}
			ifname := t.String()
			parsed[ifname] = ifaceSign{
				t:     iface,
				funcs: make(map[string]funcSign, iface.NumMethods()),
			}
			for i := 0; i < iface.NumMethods(); i++ {
				f := iface.Method(i)
				fname := f.Name()
				sign := f.Type().(*types.Signature)
				parsed[ifname].funcs[fname] = funcSign{
					params:  typeList(sign.Params()),
					results: typeList(sign.Results()),
				}
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
