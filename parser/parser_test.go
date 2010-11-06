package parser

import (
	"reflect"
	"testing"

	. "goprotobuf.googlecode.com/hg/compiler/descriptor"
	"goprotobuf.googlecode.com/hg/proto"
)

// tryParse attempts to parse the input, and verifies that it matches
// the FileDescriptorProto represented in text format.
func tryParse(t *testing.T, input, output string) {
	expected := new(FileDescriptorProto)
	if err := proto.UnmarshalText(output, expected); err != nil {
		t.Fatalf("Test failure parsing an expected proto: %v", err)
	}

	actual := new(FileDescriptorProto)
	p := newParser(input)
	if pe := p.readFile(actual); pe != nil {
		t.Errorf("Failed parsing input: %v", pe)
		return
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Mismatch! Expected:\n%v\nActual\n%v",
			proto.CompactTextString(expected), proto.CompactTextString(actual))
	}
}

type parseTest struct {
	name            string
	input, expected string
}

var parseTests = []parseTest{
	{
		"SimpleMessage",
		"message TestMessage {\n  required int32 foo = 1;\n}\n",
		`message_type { name: "TestMessage" field { name:"foo" label:LABEL_REQUIRED type:TYPE_INT32 number:1 } }`,
	},
	{
		"SimpleFields",
		"message TestMessage {\n  required int32 foo = 15;\n  optional int32 bar = 34;\n  repeated int32 baz = 3;\n}\n",
		`message_type {
		   name: "TestMessage"
		   field { name:"foo" label:LABEL_REQUIRED type:TYPE_INT32 number:15 }
		   field { name:"bar" label:LABEL_OPTIONAL type:TYPE_INT32 number:34 }
		   field { name:"baz" label:LABEL_REPEATED type:TYPE_INT32 number:3  }
		 }`,
	},
	{
		"NestedMessage",
		"message TestMessage {\n  message Nested {}\n  optional Nested test_nested = 1;\n  }\n",
		`message_type { name: "TestMessage" nested_type { name: "Nested" } field { name:"test_nested" label:LABEL_OPTIONAL number:1 type_name: "Nested" } }`,
	},
	{
		"EnumValues",
		"enum TestEnum {\n  FOO = 13;\n  BAR = -10;\n  BAZ = 500;\n}\n",
		`enum_type { name: "TestEnum" value { name:"FOO" number:13 } value { name:"BAR" number:-10 } value { name:"BAZ" number:500 } }`,
	},
}

func TestParsing(t *testing.T) {
	for _, pt := range parseTests {
		t.Logf("[ %v ]", pt.name)
		tryParse(t, pt.input, pt.expected)
	}
}
