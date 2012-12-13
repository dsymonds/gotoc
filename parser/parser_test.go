package parser

import (
	"testing"

	"code.google.com/p/goprotobuf/proto"
	. "code.google.com/p/goprotobuf/protoc-gen-go/descriptor"
)

// tryParse attempts to parse the input, and verifies that it matches
// the FileDescriptorProto represented in text format.
func tryParse(t *testing.T, input, output string) {
	want := new(FileDescriptorProto)
	if err := proto.UnmarshalText(output, want); err != nil {
		t.Fatalf("Test failure parsing a wanted proto: %v", err)
	}

	got := new(FileDescriptorProto)
	p := newParser(input)
	if pe := p.readFile(got); pe != nil {
		t.Errorf("Failed parsing input: %v", pe)
		return
	}

	if !proto.Equal(got, want) {
		t.Errorf("Mismatch!\nGot:\n%v\nWant:\n%v", got, want)
	}
}

type parseTest struct {
	name            string
	input, expected string
}

// used to shorten the FieldDefaults expected output.
const fieldDefaultsEtc = `name:"foo" label:LABEL_REQUIRED number:1`

var parseTests = []parseTest{
	{
		"SimpleMessage",
		"message TestMessage {\n  required int32 foo = 1;\n}\n",
		`message_type { name: "TestMessage" field { name:"foo" label:LABEL_REQUIRED type:TYPE_INT32 number:1 } }`,
	},
	{
		"ExplicitSyntaxIdentifier",
		"syntax = \"proto2\";\nmessage TestMessage {\n  required int32 foo = 1;\n}\n",
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
		`message TestMessage {
		  required int32  foo = 1 [default=  1  ];
		  required int32  foo = 1 [default= -2  ];
		  required int64  foo = 1 [default=  3  ];
		  required int64  foo = 1 [default= -4  ];
		  required uint32 foo = 1 [default=  5  ];
		  required uint64 foo = 1 [default=  6  ];
		  required float  foo = 1 [default=  7.5];
		  required float  foo = 1 [default= -8.5];
		  required float  foo = 1 [default=  9  ];
		  required double foo = 1 [default= 10.5];
		  required double foo = 1 [default=-11.5];
		  required double foo = 1 [default= 12  ];
		  required double foo = 1 [default= inf ];
		  required double foo = 1 [default=-inf ];
		  required double foo = 1 [default= nan ];
		  // TODO: uncomment these when the string parser handles them.
		  //required string foo = 1 [default='13\\001'];
		  //required string foo = 1 [default='a' "b" 
		  //"c"];
		  //required bytes  foo = 1 [default='14\\002'];
		  //required bytes  foo = 1 [default='a' "b" 
		  //'c'];
		  required bool   foo = 1 [default=true ];
		  required Foo    foo = 1 [default=FOO  ];
		  required int32  foo = 1 [default= 0x7FFFFFFF];
		  required int32  foo = 1 [default=-0x80000000];
		  required uint32 foo = 1 [default= 0xFFFFFFFF];
		  required int64  foo = 1 [default= 0x7FFFFFFFFFFFFFFF];
		  required int64  foo = 1 [default=-0x8000000000000000];
		  required uint64 foo = 1 [default= 0xFFFFFFFFFFFFFFFF];
		}`,
		`message_type {
		  name: "TestMessage"
		  field { type:TYPE_INT32   default_value:"1"         ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT32   default_value:"-2"        ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT64   default_value:"3"         ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT64   default_value:"-4"        ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_UINT32  default_value:"5"         ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_UINT64  default_value:"6"         ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_FLOAT   default_value:"7.5"       ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_FLOAT   default_value:"-8.5"      ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_FLOAT   default_value:"9"         ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"10.5"      ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"-11.5"     ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"12"        ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"inf"       ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"-inf"      ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_DOUBLE  default_value:"nan"       ` + fieldDefaultsEtc + ` }
		  ` +
			/*
			  field { type:TYPE_STRING  default_value:"13\\001"   ` + fieldDefaultsEtc + ` }
			  field { type:TYPE_STRING  default_value:"abc"       ` + fieldDefaultsEtc + ` }
			  field { type:TYPE_BYTES   default_value:"14\\\\002" ` + fieldDefaultsEtc + ` }
			*/
			`
		  field { type:TYPE_BOOL    default_value:"true"      ` + fieldDefaultsEtc + ` }
		  field { type_name:"Foo" default_value:"FOO"       ` + fieldDefaultsEtc + ` }

		  field { type:TYPE_INT32   default_value:"2147483647"           ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT32   default_value:"-2147483648"          ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_UINT32  default_value:"4294967295"           ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT64   default_value:"9223372036854775807"  ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_INT64   default_value:"-9223372036854775808" ` + fieldDefaultsEtc + ` }
		  field { type:TYPE_UINT64  default_value:"18446744073709551615" ` + fieldDefaultsEtc + ` }
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
	{
		"ParsePublicImports",
		"import \"foo.proto\";\nimport public \"bar.proto\";\nimport \"baz.proto\";\nimport public \"qux.proto\";\n",
		`dependency: "foo.proto" dependency: "bar.proto" dependency: "baz.proto" dependency: "qux.proto" public_dependency: 1 public_dependency: 3`,
	},
}

func TestParsing(t *testing.T) {
	for _, pt := range parseTests {
		t.Logf("[ %v ]", pt.name)
		tryParse(t, pt.input, pt.expected)
	}
}
