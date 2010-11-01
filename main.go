package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"goprotobuf.googlecode.com/hg/proto"
)

var (
	// Flags
	helpShort = flag.Bool("h", false, "Show usage text (same as --help).")
	helpLong  = flag.Bool("help", false, "Show usage text (same as -h).")

	pluginBinary = flag.String("plugin", "protoc-gen-go", "The code generator plugin to use.")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	if *helpShort || *helpLong || flag.NArg() == 0 {
		flag.Usage()
	}

	fds, err := ParseFiles(flag.Args())
	if err != nil {
		log.Exitf("Failed parsing: %v", err)
	}

	// TODO: run plugin
	fmt.Println("-----")
	proto.MarshalText(os.Stdout, fds)
	fmt.Println("-----")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:  %s [options] <foo.proto> ...\n", os.Args[0])
	flag.PrintDefaults()
}
