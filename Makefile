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

MINI_FSET_GOTOC=_mini_fset_gotoc.txt
MINI_FSET_PROTOC=_mini_fset_protoc.txt
baselinediff:
	go build
	# First, gotoc
	@./gotoc --descriptor_only testdata/mini.proto > $(MINI_FSET_GOTOC)
	@sed -i '' 's/: </ {/g' $(MINI_FSET_GOTOC)
	@sed -i '' 's/>$$/}/g' $(MINI_FSET_GOTOC)
	# Next, protoc
	@protoc --descriptor_set_out=$(MINI_TMP) --include_imports testdata/mini.proto
	@protoc --decode=google.protobuf.FileDescriptorSet -I $(PROTOBUF) $(PROTOBUF)/src/google/protobuf/descriptor.proto < $(MINI_TMP) > $(MINI_FSET_PROTOC)
	# Compare
	diff -ud $(MINI_FSET_PROTOC) $(MINI_FSET_GOTOC)
	@rm $(MINI_TMP) $(MINI_FSET_GOTOC) $(MINI_FSET_PROTOC)
