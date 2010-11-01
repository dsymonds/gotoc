package main

import (
	"io/ioutil"
	"os"

	. "goprotobuf.googlecode.com/hg/compiler/descriptor"
	"goprotobuf.googlecode.com/hg/proto"
)

func ParseFiles(filenames []string) (*FileDescriptorSet, os.Error) {
	fds := &FileDescriptorSet{
		File: make([]*FileDescriptorProto, len(filenames)),
	}

	for i, filename := range filenames {
		var err os.Error
		fds.File[i], err = parseFile(filename)
		if err != nil {
			return nil, err
		}
	}

	return fds, nil
}

func parseFile(filename string) (*FileDescriptorProto, os.Error) {
	fd := &FileDescriptorProto{
		Name: proto.String(filename),
	}

	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	// TODO
	_ = buf
	return fd, nil
}
