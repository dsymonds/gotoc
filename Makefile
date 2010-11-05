include $(GOROOT)/src/Make.inc

TARG=gotoc
GOFILES=\
	main.go\
	parser.go\

include $(GOROOT)/src/Make.cmd

minitest: $(TARG)
	./$(TARG) testdata/mini.proto
