package resolver

// TODO: The resolution implementation here is horribly inefficient.
// It may be worth optimising it at some point.

import (
	"log"
	"strings"

	. "goprotobuf.googlecode.com/hg/compiler/descriptor"
	"goprotobuf.googlecode.com/hg/proto"
)

// TODO: signal errors cleanly?
func ResolveSymbols(fds *FileDescriptorSet) {
	r := &resolver{
		fds:       fds,
	}
	for _, fd := range fds.File {
		r.resolveFile(fd)
	}
}

// A scope represents the context of the traversal.
type scope struct {
	// Valid objects are:
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
func (s *scope) findName(name string) interface{} {
	o := s.last()
	if o == nil {
		return nil
	}
	switch ov := o.(type) {
	case *FileDescriptorProto:
		for _, d := range ov.MessageType {
			if *d.Name == name {
				return d
			}
		}
		for _, e := range ov.EnumType {
			if *e.Name == name {
				return e
			}
		}
	case *DescriptorProto:
		for _, d := range ov.NestedType {
			if *d.Name == name {
				return d
			}
		}
		for _, e := range ov.EnumType {
			if *e.Name == name {
				return e
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
	fds       *FileDescriptorSet
}

func (r *resolver) resolveFile(fd *FileDescriptorProto) {
	s := new(scope)
	s.push(fd)

	// Resolve messages.
	for _, d := range fd.MessageType {
		r.resolveMessage(s, d)
	}

	// TODO: resolve other file-level types.
}

func (r *resolver) resolveMessage(s *scope, d *DescriptorProto) {
	ms := s.dup()
	ms.push(d)

	// Resolve fields.
	for _, fd := range d.Field {
		if fd.Type != nil {
			if *fd.Type != FieldDescriptorProto_TYPE_MESSAGE && *fd.Type != FieldDescriptorProto_TYPE_ENUM {
				continue
			}
		}
		o := r.resolveName(ms, *fd.TypeName)
		if o == nil {
			log.Printf("Failed to resolve name %q", *fd.TypeName)
			continue
		}
		switch ov := o.last().(type) {
		case *DescriptorProto:
			fd.Type = NewFieldDescriptorProto_Type(FieldDescriptorProto_TYPE_MESSAGE)
		case *EnumDescriptorProto:
			fd.Type = NewFieldDescriptorProto_Type(FieldDescriptorProto_TYPE_ENUM)
		}
		//log.Printf("(resolved %q to %q)", *fd.TypeName, o.fullName())
		fd.TypeName = proto.String(o.fullName())
	}
}

func (r *resolver) resolveName(s *scope, name string) *scope {
	parts := strings.Split(name, ".", -1)

	// Move up the scope, finding a place where the name makes sense.
	for ws := s.dup(); !ws.global(); ws.pop() {
		os := ws.dup()
		for _, part := range parts {
			o := os.findName(part)
			if o == nil {
				os = nil
				break
			}
			os.push(o)
		}
		if os != nil {
			// woo!
			return os
		}
	}

	// failed
	return nil
}
