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

func TestNestedMessage(t *testing.T) {
	tryParse(t,
`message TestMessage {
	message Nested {}
	optional Nested test_nested = 1;
}
`, `
message_type {
  name: "TestMessage"
  nested_type { name: "Nested" }
  field { name:"test_nested" label:LABEL_OPTIONAL number:1 type_name: "Nested" }
}
`)
}
