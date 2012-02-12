package parser

import (
	"reflect"
	"testing"

	. "code.google.com/p/goprotobuf/compiler/descriptor"
	"code.google.com/p/goprotobuf/proto"
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
		t.Errorf("Mismatch!\nExpected:\n%v\nActual\n%v",
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
		"FieldDefaults",
		"message TestMessage {\n  required string foo = 1 [default=\"blah\"];\n  required Foo    foo = 1 [default=FOO  ];\n}\n",
		`message_type {
		  name: "TestMessage"
		  field { name:"foo" label:LABEL_REQUIRED type:TYPE_STRING number:1 default_value:"blah" }
		  field { name:"foo" label:LABEL_REQUIRED type_name:"Foo" number:1 default_value:"FOO" }
		}`,
	},
	{
		"NestedMessage",
		"message TestMessage {\n  message Nested {}\n  optional Nested test_nested = 1;\n  }\n",
		`message_type { name: "TestMessage" nested_type { name: "Nested" } field { name:"test_nested" label:LABEL_OPTIONAL number:1 type_name: "Nested" } }`,
	},
	{
		"NestedEnum",
		"message TestMessage {\n  enum NestedEnum {}\n  optional NestedEnum test_enum = 1;\n  }\n",
		`message_type { name: "TestMessage" enum_type { name: "NestedEnum" } field { name:"test_enum" label:LABEL_OPTIONAL number:1 type_name: "NestedEnum" } }`,
	},
	{
		"ExtensionRange",
		"message TestMessage {\n  extensions 10 to 19;\n  extensions 30 to max;\n}\n",
		`message_type { name: "TestMessage" extension_range { start:10 end:20 } extension_range { start:30 end:536870912 } }`,
	},
	{
		"EnumValues",
		"enum TestEnum {\n  FOO = 13;\n  BAR = -10;\n  BAZ = 500;\n}\n",
		`enum_type { name: "TestEnum" value { name:"FOO" number:13 } value { name:"BAR" number:-10 } value { name:"BAZ" number:500 } }`,
	},
	{
		"ParseImport",
		"import \"foo/bar/baz.proto\";\n",
		`dependency: "foo/bar/baz.proto"`,
	},
	{
		"ParsePackage",
		"package foo.bar.baz;\n",
		`package: "foo.bar.baz"`,
	},
	{
		"ParsePackageWithSpaces",
		"package foo   .   bar.  \n  baz;\n",
		`package: "foo.bar.baz"`,
	},
	{
		"ParseFileOptions",
		"option java_package = \"com.google.foo\";\noption optimize_for = CODE_SIZE;",
		`options { uninterpreted_option { name { name_part: "java_package" is_extension: false } string_value: "com.google.foo"} uninterpreted_option { name { name_part: "optimize_for" is_extension: false } identifier_value: "CODE_SIZE" } }`,
	},
}

func TestParsing(t *testing.T) {
	for _, pt := range parseTests {
		t.Logf("[ %v ]", pt.name)
		tryParse(t, pt.input, pt.expected)
	}
}
