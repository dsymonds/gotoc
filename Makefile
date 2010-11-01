include $(GOROOT)/src/Make.inc

TARG=gotoc
GOFILES=\
	main.go\
	parser.go\

include $(GOROOT)/src/Make.cmd

test: $(TARG)
	./$(TARG) testdata/mini.proto
