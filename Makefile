minitest:
	go build
	./gotoc testdata/mini.proto

minidiff:
	go build
	./gotoc testdata/mini.proto
	mv testdata/mini.pb.go testdata/mini-gotoc.pb.go
	protoc --go_out=. testdata/mini.proto
	mv testdata/mini.pb.go testdata/mini-protoc.pb.go
	sed -i '' '/^var fileDescriptor/,/^}$$/d' testdata/mini-{gotoc,protoc}.pb.go
	diff -ud testdata/mini-{gotoc,protoc}.pb.go || true

regtest:
	go build
	testdata/run.sh

PROTOBUF=$(HOME)/src/protobuf
MINI_TMP=_mini.pb
baseline:
	@protoc --descriptor_set_out=$(MINI_TMP) --include_imports testdata/mini.proto
	@protoc --decode=google.protobuf.FileDescriptorSet -I $(PROTOBUF) $(PROTOBUF)/src/google/protobuf/descriptor.proto < $(MINI_TMP)
	@rm $(MINI_TMP)
