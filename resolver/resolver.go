package resolver

// TODO: The resolution implementation here is quite inefficient.
// It may be worth optimising it at some point.

import (
	"fmt"
	"log"

	"strings"

	"code.google.com/p/goprotobuf/proto"
	. "code.google.com/p/goprotobuf/protoc-gen-go/descriptor"
)

var _ = log.Print

func ResolveSymbols(fds *FileDescriptorSet) error {
	r := &resolver{
		fds: fds,
	}
	s := new(scope)
	s.push(fds)
	for _, fd := range fds.File {
		if err := r.resolveFile(s, fd); err != nil {
			return err
		}
	}
	return nil
}

// A scope represents the context of the traversal.
type scope struct {
	// Valid objects are:
	//	FileDescriptorSet
	//	FileDescriptorProto
	//	DescriptorProto
	//	EnumDescriptorProto
	objects []interface{}
}

func (s *scope) global() bool {
	return len(s.objects) == 0
}

func (s *scope) push(o interface{}) {
	s.objects = append(s.objects, o)
}

func (s *scope) pop() {
	s.objects = s.objects[:len(s.objects)-1]
}

func (s *scope) dup() *scope {
	sc := &scope{
		objects: make([]interface{}, len(s.objects)),
	}
	copy(sc.objects, s.objects)
	return sc
}

func (s *scope) last() interface{} {
	if s.global() {
		return nil
	}
	return s.objects[len(s.objects)-1]
}

// findName attemps to find the given name in the scope.
// Only immediate names are found; it does not recurse.
func (s *scope) findName(name string) []interface{} {
	o := s.last()
	if o == nil {
		return nil
	}
	switch ov := o.(type) {
	case *FileDescriptorSet:
		ret := []interface{}{}
		for _, fd := range ov.File {
			if fd.GetPackage() == "" {
				// No package; match on message/enum names
				fs := s.dup()
				fs.push(fd)
				ret = append(ret, fs.findName(name)...)
			} else {
				// Match on package name
				// TODO: fix this for dotted package names
				if *fd.Package == name {
					return []interface{}{fd}
				}
			}
		}
		return ret
	case *FileDescriptorProto:
		for _, d := range ov.MessageType {
			if *d.Name == name {
				return []interface{}{d}
			}
		}
		for _, e := range ov.EnumType {
			if *e.Name == name {
				return []interface{}{e}
			}
		}
	case *DescriptorProto:
		for _, d := range ov.NestedType {
			if *d.Name == name {
				return []interface{}{d}
			}
		}
		for _, e := range ov.EnumType {
			if *e.Name == name {
				return []interface{}{e}
			}
		}
		// can't be *EnumDescriptorProto
	}
	return nil
}

func (s *scope) fullName() string {
	n := make([]string, 0, len(s.objects))
	for _, o := range s.objects {
		switch ov := o.(type) {
		case *FileDescriptorProto:
			if ov.Package != nil {
				n = append(n, *ov.Package)
			}
		case *DescriptorProto:
			n = append(n, *ov.Name)
		case *EnumDescriptorProto:
			n = append(n, *ov.Name)
		}
	}
	return "." + strings.Join(n, ".")
}

type resolver struct {
	fds *FileDescriptorSet
}

func (r *resolver) resolveFile(s *scope, fd *FileDescriptorProto) error {
	fs := s.dup()
	fs.push(fd)

	// Resolve messages.
	for _, d := range fd.MessageType {
		if err := r.resolveMessage(fs, d); err != nil {
			return fmt.Errorf("(%v): %v", *fd.Name, err)
		}
	}

	// TODO: resolve other file-level types.

	return nil
}

func (r *resolver) resolveMessage(s *scope, d *DescriptorProto) error {
	ms := s.dup()
	ms.push(d)

	// Resolve fields.
	for _, fd := range d.Field {
		if fd.Type != nil {
			switch *fd.Type {
			case FieldDescriptorProto_TYPE_MESSAGE,
				FieldDescriptorProto_TYPE_GROUP,
				FieldDescriptorProto_TYPE_ENUM:
			default:
				continue
			}
		}
		o := r.resolveName(ms, *fd.TypeName)
		if o == nil {
			return fmt.Errorf("failed to resolve name %q", *fd.TypeName)
		}
		if fd.Type == nil { // TYPE_GROUP will already have this set.
			switch o.last().(type) {
			case *DescriptorProto:
				fd.Type = FieldDescriptorProto_TYPE_MESSAGE.Enum()
			case *EnumDescriptorProto:
				fd.Type = FieldDescriptorProto_TYPE_ENUM.Enum()
			}
		}
		//log.Printf("(resolved %q to %q)", *fd.TypeName, o.fullName())
		fd.TypeName = proto.String(o.fullName())
	}
	return nil
}

func (r *resolver) resolveName(s *scope, name string) *scope {
	parts := strings.Split(name, ".")

	// Move up the scope, finding a place where the name makes sense.
	for ws := s.dup(); !ws.global(); ws.pop() {
		//log.Printf("Trying to resolve %q in %q", name, ws.fullName())
		if os := matchNameComponents(ws, parts); os != nil {
			return os
		}
	}

	// failed
	return nil
}

func matchNameComponents(s *scope, parts []string) *scope {
	first, rem := parts[0], parts[1:]
	for _, o := range s.findName(first) {
		os := s.dup()
		os.push(o)
		if len(rem) == 0 {
			return os
		}
		// TODO: catch ambiguous names here
		if is := matchNameComponents(os, rem); is != nil {
			return is
		}
	}
	return nil
}
