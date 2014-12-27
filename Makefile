minitest:
	go build
	./gotoc testdata/mini.proto

regtest:
	go build
	testdata/run.sh

PROTOBUF=$(HOME)/src/protobuf
MINI_TMP=_mini.pb
baseline:
	@protoc --descriptor_set_out=$(MINI_TMP) --include_imports testdata/mini.proto
	@protoc --decode=google.protobuf.FileDescriptorSet -I $(PROTOBUF) $(PROTOBUF)/src/google/protobuf/descriptor.proto < $(MINI_TMP)
	@rm $(MINI_TMP)
