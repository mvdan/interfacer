// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mvdan/interfacer"
)

var _ = flag.Bool("v", false, "print the names of packages as they are checked")

func main() {
	flag.Parse()
	warns, err := interfacer.CheckArgs(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, warn := range warns {
		fmt.Println(warn)
	}
}
