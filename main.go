package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	// Flags
	helpShort = flag.Bool("h", false, "Show usage text (same as --help).")
	helpLong = flag.Bool("help", false, "Show usage text (same as -h).")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	if *helpShort || *helpLong || flag.NArg() == 0 {
		flag.Usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:  %s [options] <foo.proto> ...\n", os.Args[0])
	flag.PrintDefaults()
}
