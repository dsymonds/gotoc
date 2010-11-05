package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	plugin "goprotobuf.googlecode.com/hg/compiler/plugin"
	"goprotobuf.googlecode.com/hg/proto"

	"gotoc/parser"
)

var (
	// Flags
	helpShort = flag.Bool("h", false, "Show usage text (same as --help).")
	helpLong  = flag.Bool("help", false, "Show usage text (same as -h).")

	pluginBinary = flag.String("plugin", "protoc-gen-go", "The code generator plugin to use.")
)

func fullPath(binary string, paths []string) string {
	if strings.Index(binary, "/") >= 0 {
		// path with path component
		return binary
	}
	for _, p := range paths {
		full := path.Join(p, binary)
		fi, err := os.Stat(full)
		if err == nil && fi.IsRegular() {
			return full
		}
	}
	return ""
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *helpShort || *helpLong || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	fds, err := parser.ParseFiles(flag.Args())
	if err != nil {
		log.Exitf("Failed parsing: %v", err)
	}
	fmt.Println("-----")
	proto.MarshalText(os.Stdout, fds)
	fmt.Println("-----")

	// Find plugin.
	pluginPath := fullPath(*pluginBinary, strings.Split(os.Getenv("PATH"), ":", -1))
	if pluginPath == "" {
		log.Exitf("Failed finding plugin binary %q", *pluginBinary)
	}

	// Start plugin subprocess.
	pluginIn, meOut, err := os.Pipe()
	if err != nil {
		log.Exitf("Failed creating pipe: %v", err)
	}
	meIn, pluginOut, err := os.Pipe()
	if err != nil {
		log.Exitf("Failed creating pipe: %v", err)
	}
	pid, err := os.ForkExec(pluginPath, nil, nil, "/", []*os.File{pluginIn, pluginOut, os.Stderr})
	if err != nil {
		log.Exitf("Failed forking plugin: %v", err)
	}
	pluginIn.Close()
	pluginOut.Close()

	// Send request.
	cgRequest := &plugin.CodeGeneratorRequest{
		FileToGenerate: flag.Args(),
		// TODO: proto_file should be topologically sorted (bottom-up)
		ProtoFile:      fds.File,
	}
	buf, err := proto.Marshal(cgRequest)
	if err != nil {
		log.Exitf("Failed marshaling CG request: %v", err)
	}
	_, err = meOut.Write(buf)
	if err != nil {
		log.Exitf("Failed writing CG request: %v", err)
	}
	meOut.Close()

	w, err := os.Wait(pid, 0)
	if err != nil {
		log.Exitf("Failed waiting for plugin: %v", err)
	}
	if w.ExitStatus() != 0 {
		log.Exitf("Plugin exited with status %d", w.ExitStatus())
	}

	// Read response.
	cgResponse := new(plugin.CodeGeneratorResponse)
	if buf, err = ioutil.ReadAll(meIn); err != nil {
		log.Exitf("Failed reading CG response: %v", err)
	}
	if err = proto.Unmarshal(buf, cgResponse); err != nil {
		log.Exitf("Failed unmarshaling CG response: %v", err)
	}

	// TODO: check cgResponse.Error

	// TODO: write files
	for _, f := range cgResponse.File {
		fmt.Printf("--[ %v ]--\n", proto.GetString(f.Name))
		fmt.Println(proto.GetString(f.Content))
	}
	fmt.Println("-----")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:  %s [options] <foo.proto> ...\n", os.Args[0])
	flag.PrintDefaults()
}
