include $(GOROOT)/src/Make.inc

TARG=protocmp
GOFILES=\
	protocmp.go\

CLEANFILES+=*.actual

include $(GOROOT)/src/Make.cmd

test: protocmp
	./run.sh
