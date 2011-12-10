package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	plugin "goprotobuf.googlecode.com/hg/compiler/plugin"
	"goprotobuf.googlecode.com/hg/proto"

	"github.com/dsymonds/gotoc/parser"
	"github.com/dsymonds/gotoc/resolver"
)

var (
	// Flags
	helpShort = flag.Bool("h", false, "Show usage text (same as --help).")
	helpLong  = flag.Bool("help", false, "Show usage text (same as -h).")

	importPath     = flag.String("import_path", ".", "Comma-separated list of paths to search for imports.")
	pluginBinary   = flag.String("plugin", "protoc-gen-go", "The code generator plugin to use.")
	descriptorOnly = flag.Bool("descriptor_only", false, "Whether to print out only the FileDescriptorSet.")
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

	fds, err := parser.ParseFiles(flag.Args(), strings.Split(*importPath, ","))
	if err != nil {
		log.Fatalf("Failed parsing: %v", err)
	}
	if err := resolver.ResolveSymbols(fds); err != nil {
		log.Fatalf("Failed resolving symbols: %v", err)
	}

	if *descriptorOnly {
		proto.MarshalText(os.Stdout, fds)
		os.Exit(0)
	}

	fmt.Println("-----")
	proto.MarshalText(os.Stdout, fds)
	fmt.Println("-----")

	// Prepare request.
	cgRequest := &plugin.CodeGeneratorRequest{
		FileToGenerate: flag.Args(),
		// TODO: proto_file should be topologically sorted (bottom-up)
		ProtoFile: fds.File,
	}
	buf, err := proto.Marshal(cgRequest)
	if err != nil {
		log.Fatalf("Failed marshaling CG request: %v", err)
	}

	// Find plugin.
	pluginPath := fullPath(*pluginBinary, strings.Split(os.Getenv("PATH"), ":"))
	if pluginPath == "" {
		log.Fatalf("Failed finding plugin binary %q", *pluginBinary)
	}

	// Run the plugin subprocess.
	cmd := &exec.Cmd{
		Path:   pluginPath,
		Stdin:  bytes.NewBuffer(buf),
		Stderr: os.Stderr,
	}
	buf, err = cmd.Output()
	if err != nil {
		log.Fatalf("Failed running plugin: %v", err)
	}

	// Parse the response.
	cgResponse := new(plugin.CodeGeneratorResponse)
	if err = proto.Unmarshal(buf, cgResponse); err != nil {
		log.Fatalf("Failed unmarshaling CG response: %v", err)
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
