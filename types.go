/* Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc> */
/* See LICENSE for licensing information */

package main

import (
	"go/importer"
	"go/types"
	"regexp"
)

var pkgs = [...]string{
	"crypto",
	"encoding",
	"encoding/binary",
	"encoding/gob",
	"encoding/json",
	"encoding/xml",
	"flag",
	"fmt",
	"hash",
	"image",
	"io",
	"net",
	"net/http",
	"os",
	"reflect",
	"runtime",
	"sort",
	"sync",
}

type funcSign struct {
	params  []types.Type
	results []types.Type
}

type ifaceSign struct {
	t *types.Interface

	funcs map[string]funcSign
}

type cache struct {
	done map[string]struct{}

	stdIfaces map[string]ifaceSign
	ownIfaces map[string]ifaceSign
}

func typesInit() error {
	c = &cache{
		done:      make(map[string]struct{}),
		stdIfaces: make(map[string]ifaceSign),
		ownIfaces: make(map[string]ifaceSign),
	}
	imp := importer.Default()
	for _, path := range pkgs {
		pkg, err := imp.Import(path)
		if err != nil {
			return err
		}
		c.grabFromScope(pkg.Scope(), false, false, path)
	}
	c.grabFromScope(types.Universe, false, true, "")
	return nil
}

var exported = regexp.MustCompile(`^[A-Z]`)

func (c *cache) grabFromScope(scope *types.Scope, own, unexported bool, impPath string) {
	ifaces := c.stdIfaces
	if own {
		ifaces = c.ownIfaces
	}
	c.done[impPath] = struct{}{}
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if !unexported && !exported.MatchString(tn.Name()) {
			continue
		}
		t := tn.Type()
		if impPath != "" && impPath[0] != '.' {
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
