/*
Package gendesc generates descriptor protos from an AST.
*/
package gendesc

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dsymonds/gotoc/ast"
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
	for _, i := range f.PublicImports {
		fdp.PublicDependency = append(fdp.PublicDependency, int32(i))
	}
	sort.Sort(int32Slice(fdp.PublicDependency))
	for _, m := range f.Messages {
		dp, err := genMessage(m)
		if err != nil {
			return nil, err
		}
		fdp.MessageType = append(fdp.MessageType, dp)
	}
	for _, enum := range f.Enums {
		edp, err := genEnum(enum)
		if err != nil {
			return nil, err
		}
		fdp.EnumType = append(fdp.EnumType, edp)
	}
	for _, srv := range f.Services {
		sdp, err := genService(srv)
		if err != nil {
			return nil, err
		}
		fdp.Service = append(fdp.Service, sdp)
	}
	for _, ext := range f.Extensions {
		fdps, err := genExtension(ext)
		if err != nil {
			return nil, err
		}
		fdp.Extension = append(fdp.Extension, fdps...)
	}
	for _, opt := range f.Options {
		if fdp.Options == nil {
			fdp.Options = new(pb.FileOptions)
		}
		// TODO: interpret common options
		uo := new(pb.UninterpretedOption)
		for _, part := range strings.Split(opt[0], ".") {
			// TODO: support IsExtension
			uo.Name = append(uo.Name, &pb.UninterpretedOption_NamePart{
				NamePart:    proto.String(part),
				IsExtension: proto.Bool(false),
			})
			// TODO: need to handle more types
			if strings.HasPrefix(opt[1], `"`) {
				// TODO: doesn't handle single quote strings, etc.
				unq, err := strconv.Unquote(opt[1])
				if err != nil {
					return nil, err
				}
				uo.StringValue = []byte(unq)
			} else {
				uo.IdentifierValue = proto.String(opt[1])
			}
		}
		fdp.Options.UninterpretedOption = append(fdp.Options.UninterpretedOption, uo)
	}
	// TODO: SourceCodeInfo
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
	var extraNested []*pb.DescriptorProto
	for _, f := range m.Fields {
		fdp, xdp, err := genField(f)
		if err != nil {
			return nil, err
		}
		dp.Field = append(dp.Field, fdp)
		if xdp != nil {
			extraNested = append(extraNested, xdp)
		}
	}
	for _, ext := range m.Extensions {
		fdps, err := genExtension(ext)
		if err != nil {
			return nil, err
		}
		dp.Extension = append(dp.Extension, fdps...)
	}
	for _, nm := range m.Messages {
		ndp, err := genMessage(nm)
		if err != nil {
			return nil, err
		}
		dp.NestedType = append(dp.NestedType, ndp)
	}
	// Put extra nested DescriptorProtos (e.g. from a map field)
	// at the end so they don't disrupt message indexes.
	dp.NestedType = append(dp.NestedType, extraNested...)
	for _, ne := range m.Enums {
		edp, err := genEnum(ne)
		if err != nil {
			return nil, err
		}
		dp.EnumType = append(dp.EnumType, edp)
	}
	for _, r := range m.ExtensionRanges {
		// DescriptorProto.ExtensionRange uses a half-open interval.
		dp.ExtensionRange = append(dp.ExtensionRange, &pb.DescriptorProto_ExtensionRange{
			Start: proto.Int32(int32(r[0])),
			End:   proto.Int32(int32(r[1] + 1)),
		})
	}
	for _, oo := range m.Oneofs {
		dp.OneofDecl = append(dp.OneofDecl, &pb.OneofDescriptorProto{
			Name: proto.String(oo.Name),
		})
	}
	return dp, nil
}

func genField(f *ast.Field) (*pb.FieldDescriptorProto, *pb.DescriptorProto, error) {
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
	if f.KeyTypeName != "" {
		mname := camelCase(f.Name) + "Entry"
		vmsg := &ast.Message{
			Name: mname,
			Fields: []*ast.Field{
				{
					TypeName: f.KeyTypeName,
					Type:     f.KeyType,
					Name:     "key",
					Tag:      1,
				},
				{
					TypeName: f.TypeName,
					Type:     f.Type,
					Name:     "value",
					Tag:      2,
				},
			},
			Up: f.Up,
		}
		vmsg.Fields[0].Up = vmsg
		vmsg.Fields[1].Up = vmsg
		xdp, err := genMessage(vmsg)
		if err != nil {
			return nil, nil, fmt.Errorf("internal error: %v", err)
		}
		xdp.Options = &pb.MessageOptions{
			MapEntry: proto.Bool(true),
		}
		fdp.Type = pb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		fdp.TypeName = proto.String(qualifiedName(vmsg))
		return fdp, xdp, nil
	}
	switch t := f.Type.(type) {
	case ast.FieldType:
		pt, ok := fieldTypeMap[t]
		if !ok {
			return nil, nil, fmt.Errorf("internal error: no mapping from ast.FieldType %v", t)
		}
		fdp.Type = pt.Enum()
	case *ast.Message:
		if !t.Group {
			fdp.Type = pb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		} else {
			fdp.Type = pb.FieldDescriptorProto_TYPE_GROUP.Enum()
			// The field name is lowercased by protoc.
			*fdp.Name = strings.ToLower(*fdp.Name)
		}
		fdp.TypeName = proto.String(qualifiedName(t))
	case *ast.Enum:
		fdp.Type = pb.FieldDescriptorProto_TYPE_ENUM.Enum()
		fdp.TypeName = proto.String(qualifiedName(t))
	default:
		return nil, nil, fmt.Errorf("internal error: bad ast.Field.Type type %T", f.Type)
	}
	if ext, ok := f.Up.(*ast.Extension); ok {
		fdp.Extendee = proto.String(qualifiedName(ext.ExtendeeType))
	}
	if f.HasDefault {
		fdp.DefaultValue = proto.String(f.Default)
	}
	if f.Oneof != nil {
		n := 0
		for _, oo := range f.Oneof.Up.Oneofs {
			if oo == f.Oneof {
				break
			}
			n++
		}
		fdp.OneofIndex = proto.Int(n)
	}

	return fdp, nil, nil
}

func genEnum(enum *ast.Enum) (*pb.EnumDescriptorProto, error) {
	edp := &pb.EnumDescriptorProto{
		Name: proto.String(enum.Name),
	}
	for _, ev := range enum.Values {
		edp.Value = append(edp.Value, &pb.EnumValueDescriptorProto{
			Name:   proto.String(ev.Name),
			Number: proto.Int32(ev.Number),
		})
	}
	return edp, nil
}

func genService(srv *ast.Service) (*pb.ServiceDescriptorProto, error) {
	sdp := &pb.ServiceDescriptorProto{
		Name: proto.String(srv.Name),
	}
	for _, mth := range srv.Methods {
		mdp, err := genMethod(mth)
		if err != nil {
			return nil, err
		}
		sdp.Method = append(sdp.Method, mdp)
	}
	return sdp, nil
}

func genMethod(mth *ast.Method) (*pb.MethodDescriptorProto, error) {
	mdp := &pb.MethodDescriptorProto{
		Name:       proto.String(mth.Name),
		InputType:  proto.String(qualifiedName(mth.InType)),
		OutputType: proto.String(qualifiedName(mth.OutType)),
	}
	return mdp, nil
}

func genExtension(ext *ast.Extension) ([]*pb.FieldDescriptorProto, error) {
	var fdps []*pb.FieldDescriptorProto
	for _, f := range ext.Fields {
		// TODO: It should be impossible to get a map field?
		fdp, _, err := genField(f)
		if err != nil {
			return nil, err
		}
		fdps = append(fdps, fdp)
	}
	return fdps, nil
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
	if f := x.(*ast.File); true {
		// Add package components in reverse order.
		for i := len(f.Package) - 1; i >= 0; i-- {
			parts = append(parts, f.Package[i])
		}
	}
	// Reverse parts, then join with dots.
	for i, j := 0, len(parts)-1; i < j; {
		parts[i], parts[j] = parts[j], parts[i]
		i++
		j--
	}
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

type int32Slice []int32

func (s int32Slice) Len() int           { return len(s) }
func (s int32Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s int32Slice) Less(i, j int) bool { return s[i] < s[j] }

// camelCase turns foo_bar into FooBar.
func camelCase(s string) string {
	words := strings.Split(s, "_")
	for i, word := range words {
		words[i] = strings.Title(word)
	}
	return strings.Join(words, "")
}
