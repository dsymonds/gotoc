include $(GOROOT)/src/Make.inc

TARG=gotoc
GOFILES=\
	main.go\

DEPS=parser

include $(GOROOT)/src/Make.cmd

test:
	make -C parser test

minitest: $(TARG)
	./$(TARG) testdata/mini.proto
