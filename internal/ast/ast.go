/*
Package ast defines the AST data structures used by gotoc.
*/
package ast

import (
	"fmt"
	"log"
	"sort"
)

// Node is implemented by concrete types that represent things appearing in a proto file.
type Node interface {
	Pos() Position
	File() *File
}

// FileSet describes a set of proto files.
type FileSet struct {
	Files []*File
}

// File represents a single proto file.
type File struct {
	Name    string // filename
	Syntax  string // "proto2" or "proto3"
	Package []string

	Imports []string

	Messages []*Message // top-level messages
	Enums    []*Enum    // top-level enums

	Comments []*Comment // all the comments for this file, sorted by position
}

// Message represents a proto message.
type Message struct {
	Position Position // position of the "message" token
	Name     string
	Fields   []*Field

	Messages []*Message

	Up interface{} // either *File or *Message
}

func (m *Message) Pos() Position { return m.Position }
func (m *Message) File() *File {
	for x := m.Up; ; {
		switch up := x.(type) {
		case *File:
			return up
		case *Message:
			x = up.Up
		default:
			log.Panicf("internal error: Message.Up is a %T", up)
		}
	}
}

// Field represents a field in a message.
type Field struct {
	Position Position // position of "required"/"optional"/"repeated"/type

	// TypeName is the raw name parsed from the input.
	// Type is set during resolution; it will be a FieldType, *Message or *Enum.
	TypeName string
	Type     interface{}

	// At most one of {required,repeated} is set.
	Required bool
	Repeated bool
	Name     string
	Tag      int

	HasDefault bool
	Default    string // e.g. "foo", 7, true

	Up *Message
}

func (f *Field) Pos() Position { return f.Position }
func (f *Field) File() *File   { return f.Up.File() }

type FieldType int8

const (
	min FieldType = iota
	Int64
	Int32
	Bool
	String
	Sint64
	max
)

func (ft FieldType) IsValid() bool { return min < ft && ft < max }

var FieldTypeMap = map[FieldType]string{
	Int64:  "int64",
	Int32:  "int32",
	Bool:   "bool",
	String: "string",
	Sint64: "sint64",
}

func (ft FieldType) String() string {
	if s, ok := FieldTypeMap[ft]; ok {
		return s
	}
	return "UNKNOWN"
}

type Enum struct {
	Position Position // position of "enum" token
	Name     string
	Values   []*EnumValue

	Up interface{} // either *File or *Message
}

func (enum *Enum) Pos() Position { return enum.Position }
func (enum *Enum) File() *File {
	for x := enum.Up; ; {
		switch up := x.(type) {
		case *File:
			return up
		case *Message:
			x = up.Up
		default:
			log.Panicf("internal error: Enum.Up is a %T", up)
		}
	}
}

type EnumValue struct {
	Position Position // position of Name
	Name     string
	Number   int32

	Up *Enum
}

func (ev *EnumValue) Pos() Position { return ev.Position }
func (ev *EnumValue) File() *File   { return ev.Up.File() }

// Comment represents a comment.
type Comment struct {
	Start, End Position // position of first and last "//"
	Text       []string
}

func (c *Comment) Pos() Position { return c.Start }

// LeadingComment returns the comment that immediately precedes a node,
// or nil if there's no such comment.
func LeadingComment(n Node) *Comment {
	f := n.File()
	// Get the comment whose End position is on the previous line.
	lineEnd := n.Pos().Line - 1
	ci := sort.Search(len(f.Comments), func(i int) bool {
		return f.Comments[i].End.Line >= lineEnd
	})
	if ci >= len(f.Comments) || f.Comments[ci].End.Line != lineEnd {
		return nil
	}
	return f.Comments[ci]
}

// InlineComment returns the comment on the same line as a node,
// or nil if there's no inline comment.
// The returned comment is guaranteed to be a single line.
func InlineComment(n Node) *Comment {
	// TODO: Do we care about comments line this?
	// 	string name = 1; /* foo
	// 	bar */

	f := n.File()
	pos := n.Pos()
	ci := sort.Search(len(f.Comments), func(i int) bool {
		return f.Comments[i].Start.Line >= pos.Line
	})
	if ci >= len(f.Comments) || f.Comments[ci].Start.Line != pos.Line {
		return nil
	}
	c := f.Comments[ci]
	// Sanity check; it should only be one line.
	if c.Start != c.End || len(c.Text) != 1 {
		log.Panicf("internal error: bad inline comment: %+v", c)
	}
	return c
}

// Position describes a source position in an input file.
// It is only valid if the line number is positive.
type Position struct {
	Line   int // 1-based line number
	Offset int // 0-based byte offset
}

func (pos Position) IsValid() bool              { return pos.Line > 0 }
func (pos Position) Before(other Position) bool { return pos.Offset < other.Offset }
func (pos Position) String() string {
	if pos.Line == 0 {
		return ":<invalid>"
	}
	return fmt.Sprintf(":%d", pos.Line)
}
