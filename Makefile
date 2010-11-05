include $(GOROOT)/src/Make.inc

TARG=gotoc
GOFILES=\
	main.go\

DEPS=parser

include $(GOROOT)/src/Make.cmd

minitest: $(TARG)
	./$(TARG) testdata/mini.proto
