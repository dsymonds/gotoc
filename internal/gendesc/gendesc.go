/*
Package gendesc generates descriptor protos from an AST.
*/
package gendesc

import (
	"fmt"
	"strings"

	"github.com/dsymonds/gotoc/internal/ast"
	"github.com/golang/protobuf/proto"
	pb "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func Generate(fs *ast.FileSet) (*pb.FileDescriptorSet, error) {
	fds := new(pb.FileDescriptorSet)
	for _, f := range fs.Files {
		fdp, err := genFile(f)
		if err != nil {
			return nil, err
		}
		fds.File = append(fds.File, fdp)
	}
	return fds, nil
}

func genFile(f *ast.File) (*pb.FileDescriptorProto, error) {
	fdp := &pb.FileDescriptorProto{
		Name:    maybeString(f.Name),
		Package: maybeString(strings.Join(f.Package, ".")),
	}
	for _, imp := range f.Imports {
		fdp.Dependency = append(fdp.Dependency, imp)
	}
	for _, m := range f.Messages {
		dp, err := genMessage(m)
		if err != nil {
			return nil, err
		}
		fdp.MessageType = append(fdp.MessageType, dp)
	}
	// TODO: EnumType, SourceCodeInfo
	switch f.Syntax {
	case "proto2", "":
		// "proto2" is considered the default; don't set anything.
	default:
		fdp.Syntax = proto.String(f.Syntax)
	}

	return fdp, nil
}

func genMessage(m *ast.Message) (*pb.DescriptorProto, error) {
	dp := &pb.DescriptorProto{
		Name: proto.String(m.Name),
	}
	for _, f := range m.Fields {
		fdp, err := genField(f)
		if err != nil {
			return nil, err
		}
		dp.Field = append(dp.Field, fdp)
	}
	for _, nm := range m.Messages {
		ndp, err := genMessage(nm)
		if err != nil {
			return nil, err
		}
		dp.NestedType = append(dp.NestedType, ndp)
	}
	// TODO: EnumType
	return dp, nil
}

func genField(f *ast.Field) (*pb.FieldDescriptorProto, error) {
	fdp := &pb.FieldDescriptorProto{
		Name:   proto.String(f.Name),
		Number: proto.Int32(int32(f.Tag)),
	}
	switch {
	case f.Required:
		fdp.Label = pb.FieldDescriptorProto_LABEL_REQUIRED.Enum()
	case f.Repeated:
		fdp.Label = pb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	default:
		// default is optional
		fdp.Label = pb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}
	switch t := f.Type.(type) {
	case ast.FieldType:
		pt, ok := fieldTypeMap[t]
		if !ok {
			return nil, fmt.Errorf("internal error: no mapping from ast.FieldType %v", t)
		}
		fdp.Type = pt.Enum()
	case *ast.Message, *ast.Enum:
		fdp.TypeName = proto.String(f.TypeName)
	default:
		return nil, fmt.Errorf("internal error: bad ast.Field.Type type %T", f.Type)
	}
	if f.HasDefault {
		fdp.DefaultValue = proto.String(f.Default)
	}

	return fdp, nil
}

// qualifiedName returns the fully-qualified name of x,
// which must be either *ast.Message or *ast.Enum.
func qualifiedName(x interface{}) string {
	var parts []string
	for {
		switch v := x.(type) {
		case *ast.Message:
			parts = append(parts, v.Name)
			x = v.Up
			continue
		case *ast.Enum:
			parts = append(parts, v.Name)
			x = v.Up
			continue
		}
		break // *ast.File
	}
	// Reverse parts, then join with dots.
	for i, j := 0, len(parts)-1; i < j; {
		parts[i], parts[j] = parts[j], parts[i]
		i++
		j--
	}
	// TODO: mix in package name?
	return "." + strings.Join(parts, ".")
}

// A mapping of ast.FieldType to the proto type.
// Does not include TYPE_ENUM, TYPE_MESSAGE or TYPE_GROUP.
var fieldTypeMap = map[ast.FieldType]pb.FieldDescriptorProto_Type{
	ast.Double:   pb.FieldDescriptorProto_TYPE_DOUBLE,
	ast.Float:    pb.FieldDescriptorProto_TYPE_FLOAT,
	ast.Int64:    pb.FieldDescriptorProto_TYPE_INT64,
	ast.Uint64:   pb.FieldDescriptorProto_TYPE_UINT64,
	ast.Int32:    pb.FieldDescriptorProto_TYPE_INT32,
	ast.Fixed64:  pb.FieldDescriptorProto_TYPE_FIXED64,
	ast.Fixed32:  pb.FieldDescriptorProto_TYPE_FIXED32,
	ast.Bool:     pb.FieldDescriptorProto_TYPE_BOOL,
	ast.String:   pb.FieldDescriptorProto_TYPE_STRING,
	ast.Bytes:    pb.FieldDescriptorProto_TYPE_BYTES,
	ast.Uint32:   pb.FieldDescriptorProto_TYPE_UINT32,
	ast.Sfixed32: pb.FieldDescriptorProto_TYPE_SFIXED32,
	ast.Sfixed64: pb.FieldDescriptorProto_TYPE_SFIXED64,
	ast.Sint32:   pb.FieldDescriptorProto_TYPE_SINT32,
	ast.Sint64:   pb.FieldDescriptorProto_TYPE_SINT64,
}

func maybeString(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}
