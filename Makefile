include $(GOROOT)/src/Make.inc

TARG=gotoc
GOFILES=\
	main.go\

DEPS=parser resolver

include $(GOROOT)/src/Make.cmd

test:
	make -C parser test

minitest: $(TARG)
	./$(TARG) testdata/mini.proto

PROTOBUF=$(HOME)/src/protobuf
MINI_TMP=_mini.pb
baseline:
	@protoc --descriptor_set_out=$(MINI_TMP) testdata/mini.proto
	@protoc --decode=google.protobuf.FileDescriptorSet -I $(PROTOBUF) $(PROTOBUF)/src/google/protobuf/descriptor.proto < $(MINI_TMP)
	@rm $(MINI_TMP)
