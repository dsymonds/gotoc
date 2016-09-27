/*
gotoc is a protocol buffer compiler. It reads and parses .proto files,
and produces output that can be consumed by a protoc-compatible plugin
(such as protoc-gen-go) to produce generated code.
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/golang/protobuf/proto"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"

	"github.com/dsymonds/gotoc/gendesc"
	"github.com/dsymonds/gotoc/parser"
)

var (
	// Flags
	helpShort = flag.Bool("h", false, "Show usage text (same as --help).")
	helpLong  = flag.Bool("help", false, "Show usage text (same as -h).")

	importPath     = flag.String("import_path", ".", "Comma-separated list of paths to search for imports.")
	pluginBinary   = flag.String("plugin", "protoc-gen-go", "The code generator plugin to use.")
	descriptorOnly = flag.Bool("descriptor_only", false, "Whether to print out only the FileDescriptorSet.")
	params         = flag.String("params", "", "Parameters to pass to the code generator plugin (plugin-specific format).")
)

func fullPath(binary string, paths []string) string {
	if strings.Index(binary, "/") >= 0 {
		// path with path component
		return binary
	}
	for _, p := range paths {
		full := path.Join(p, binary)
		fi, err := os.Stat(full)
		if err == nil && !fi.IsDir() {
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

	fs, err := parser.ParseFiles(flag.Args(), strings.Split(*importPath, ","))
	if err != nil {
		fatalf("%v", err)
	}
	fds, err := gendesc.Generate(fs)
	if err != nil {
		fatalf("Failed generating descriptors: %v", err)
	}

	if *descriptorOnly {
		proto.MarshalText(os.Stdout, fds)
		os.Exit(0)
	}

	//fmt.Println("-----")
	//proto.MarshalText(os.Stdout, fds)
	//fmt.Println("-----")

	// Prepare request.
	cgRequest := &plugin.CodeGeneratorRequest{
		FileToGenerate: flag.Args(),
		ProtoFile:      fds.File,
	}
	if *params != "" {
		cgRequest.Parameter = params
	}
	buf, err := proto.Marshal(cgRequest)
	if err != nil {
		fatalf("Failed marshaling CG request: %v", err)
	}

	// Find plugin.
	pluginPath := fullPath(*pluginBinary, strings.Split(os.Getenv("PATH"), ":"))
	if pluginPath == "" {
		fatalf("Failed finding plugin binary %q", *pluginBinary)
	}

	// Run the plugin subprocess.
	cmd := &exec.Cmd{
		Path:   pluginPath,
		Stdin:  bytes.NewBuffer(buf),
		Stderr: os.Stderr,
	}
	buf, err = cmd.Output()
	if err != nil {
		fatalf("Failed running plugin: %v", err)
	}

	// Parse the response.
	cgResponse := new(plugin.CodeGeneratorResponse)
	if err = proto.Unmarshal(buf, cgResponse); err != nil {
		fatalf("Failed unmarshaling CG response: %v", err)
	}

	// TODO: check cgResponse.Error

	for _, f := range cgResponse.File {
		// TODO: If f.Name is nil, the content should be appended to the previous file.
		if f.Name == nil || f.Content == nil {
			fatalf("Malformed CG response")
		}
		if err := ioutil.WriteFile(*f.Name, []byte(*f.Content), 0644); err != nil {
			fatalf("Failed writing output file: %v", err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:  %s [options] <foo.proto> ...\n", os.Args[0])
	flag.PrintDefaults()
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
