// A small tool to compare two text-format FileDescriptorSet protocol buffers.

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/golang/protobuf/proto"
	. "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func main() {
	flag.Parse()
	if flag.NArg() != 2 {
		log.Fatalf("usage: %v <proto1> <proto2>", os.Args[0])
	}

	a, b := mustLoad(flag.Arg(0)), mustLoad(flag.Arg(1))
	cmpSets(a, b)
}

func mustLoad(filename string) *FileDescriptorSet {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed reading %v: %v", filename, err)
	}
	fds := new(FileDescriptorSet)
	if err := proto.UnmarshalText(string(buf), fds); err != nil {
		log.Fatalf("Failed parsing %v: %v", filename, err)
	}
	return fds
}

func cmpSets(a, b *FileDescriptorSet) {
	// Index each set by filename.
	indexA, indexB := make(map[string]int), make(map[string]int)
	for i, fd := range a.File {
		indexA[*fd.Name] = i
	}
	for i, fd := range b.File {
		indexB[*fd.Name] = i
	}

	// Check that the filename sets match.
	match := true
	if len(indexA) != len(indexB) {
		match = false
	}
	for filename, _ := range indexA {
		if _, ok := indexB[filename]; !ok {
			match = false
			break
		}
	}
	for filename, _ := range indexB {
		if _, ok := indexA[filename]; !ok {
			match = false
			break
		}
	}
	if !match {
		log.Printf("Sets of filenames do not match.")
		log.Printf("A: %+v", indexA)
		log.Printf("B: %+v", indexB)
		os.Exit(1)
	}

	// TODO: could also verify that the file ordering is topological?

	for _, fdA := range a.File {
		fdB := b.File[indexB[*fdA.Name]]
		cmpFiles(fdA, fdB)
	}
}

func cmpFiles(a, b *FileDescriptorProto) {
	if ap, bp := a.GetPackage(), b.GetPackage(); ap != bp {
		log.Fatalf("Package name mismatch in %v: %q vs. %q", *a.Name, ap, bp)
	}

	match := true
	if len(a.Dependency) != len(b.Dependency) {
		match = false
	} else {
		for i, depA := range a.Dependency {
			if depA != b.Dependency[i] {
				match = false
				break
			}
		}
	}
	if !match {
		log.Fatalf("Different dependency list in %v", *a.Name)
	}

	// TODO: this should be order-independent.
	if len(a.MessageType) != len(b.MessageType) {
		log.Fatalf("Different number of messages in %v", *a.Name)
	}
	for i, msgA := range a.MessageType {
		cmpMessages(msgA, b.MessageType[i])
	}

	// TODO: enum_type
}

func cmpMessages(a, b *DescriptorProto) {
	// TODO: this check shouldn't be necessary from here.
	if *a.Name != *b.Name {
		log.Fatalf("Different message names: %q vs. %q", *a.Name, *b.Name)
	}

	// TODO: this should be order-independent.
	if len(a.Field) != len(b.Field) {
		log.Fatalf("Different number of fields in message %v: %d vs. %d", *a.Name, len(a.Field), len(b.Field))
	}
	for i, fA := range a.Field {
		cmpFields(fA, b.Field[i])
	}

	// TODO: this should be order-independent too.
	if len(a.NestedType) != len(b.NestedType) {
		log.Fatalf("Different number of nested messages in message %v: %d vs. %d",
			*a.Name, len(a.NestedType), len(b.NestedType))
	}
	for i, msgA := range a.NestedType {
		cmpMessages(msgA, b.NestedType[i])
	}

	// TODO: nested_type, enum_type
}

func cmpFields(a, b *FieldDescriptorProto) {
	// TODO: this check shouldn't be necessary from here.
	if *a.Name != *b.Name {
		log.Fatalf("Different field names: %q vs. %q", *a.Name, *b.Name)
	}
	if *a.Number != *b.Number {
		log.Fatalf("Different field number for %v: %d vs. %d", *a.Name, *a.Number, *b.Number)
	}
	if *a.Label != *b.Label {
		log.Fatalf("Different field labels for %v: %v vs. %v", *a.Name,
			FieldDescriptorProto_Label_name[int32(*a.Label)],
			FieldDescriptorProto_Label_name[int32(*b.Label)])
	}
	if *a.Type != *b.Type {
		log.Fatalf("Different field types for %v: %v vs. %v", *a.Name,
			FieldDescriptorProto_Type_name[int32(*a.Type)],
			FieldDescriptorProto_Type_name[int32(*b.Type)])
	}
	if aTN, bTN := a.GetTypeName(), b.GetTypeName(); aTN != bTN {
		log.Fatalf("Different field type_name for %v: %q vs. %q", *a.Name, aTN, bTN)
	}
	if ad, bd := a.GetDefaultValue(), b.GetDefaultValue(); ad != bd {
		log.Fatalf("Different field default_value for %v: %q vs. %q", *a.Name, ad, bd)
	}
}
