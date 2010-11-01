package main

import (
	"io/ioutil"
	"os"

	. "goprotobuf.googlecode.com/hg/compiler/descriptor"
)

func ParseFiles(filenames []string) (*FileDescriptorSet, os.Error) {
	fds := &FileDescriptorSet{
		File: make([]*FileDescriptorProto, len(filenames)),
	}

	for i, filename := range filenames {
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		fds.File[i], err = parseFile(buf)
		if err != nil {
			return nil, err
		}
	}

	return fds, nil
}

func parseFile(buf []byte) (*FileDescriptorProto, os.Error) {
	// TODO
	return nil, nil
}
