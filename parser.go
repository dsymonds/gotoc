package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	. "goprotobuf.googlecode.com/hg/compiler/descriptor"
	"goprotobuf.googlecode.com/hg/proto"
)

func ParseFiles(filenames []string) (*FileDescriptorSet, os.Error) {
	fds := &FileDescriptorSet{
		File: make([]*FileDescriptorProto, len(filenames)),
	}

	for i, filename := range filenames {
		fds.File[i] = &FileDescriptorProto{
			Name: proto.String(filename),
		}
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		p := newParser(string(buf))
		if pe := p.readFile(fds.File[i]); pe != nil {
			return nil, pe
		}
		log.Printf("Leftovers: %q", p.s)
	}

	return fds, nil
}

type parseError struct {
	message string
	line    int // 1-based line number
	offset  int // 0-based byte offset from start of input
}

func (pe *parseError) String() string {
	if pe == nil {
		return "<nil>"
	}
	if pe.line == 1 {
		return fmt.Sprintf("line 1.%d: %v", pe.offset, pe.message)
	}
	return fmt.Sprintf("line %d: %v", pe.line, pe.message)
}

type token struct {
	value        string
	err          *parseError
	line, offset int
}

type parser struct {
	s            string // remaining input
	done         bool   // whether the parsing is finished
	backed       bool   // whether back() was called
	offset, line int
	cur          token
}

func newParser(s string) *parser {
	return &parser{
		s:    s,
		line: 1,
		cur: token{
			line: 1,
		},
	}
}

func (p *parser) readFile(fd *FileDescriptorProto) *parseError {
	// Read package
	if err := p.readToken("package"); err != nil {
		return err
	}
	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	// TODO: check for a good package name
	fd.Package = proto.String(tok.value)
	if err := p.readToken(";"); err != nil {
		return err
	}

	// Parse the rest of the file.
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		switch tok.value {
		case "message":
			msg := new(DescriptorProto)
			fd.MessageType = append(fd.MessageType, msg)
			if err := p.readMessage(msg); err != nil {
				return err
			}
		default:
			return p.error("unknown top-level thing %q", tok.value)
		}
	}

	// TODO: more

	return nil
}

func (p *parser) readMessage(d *DescriptorProto) *parseError {
	// TODO
	return nil
}

func (p *parser) readToken(expected string) *parseError {
	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	if tok.value != expected {
		return p.error("expected %q, found %q", expected, tok.value)
	}
	return nil
}

// Back off the parser by one token; may only be done between calls to next().
func (p *parser) back() {
	p.backed = true
}

// Advances the parser and returns the new current token.
func (p *parser) next() *token {
	if p.backed || p.done {
		p.backed = false
	} else {
		p.advance()
		if p.done {
			p.cur.value = ""
		}
	}
	log.Printf("parser·next(): returning %q [err: %v]", p.cur.value, p.cur.err)
	return &p.cur
}

func (p *parser) advance() {
	// Skip whitespace
	p.skipWhitespaceAndComments()
	if p.done {
		return
	}

	// Start of non-whitespace
	p.cur.err = nil
	p.cur.offset, p.cur.line = p.offset, p.line
	switch p.s[0] {
	// TODO: more cases, like punctuation.
	case ';':
		// Single symbol
		p.cur.value, p.s = p.s[:1], p.s[1:]
	default:
		i := 0
		for i < len(p.s) && isIdentOrNumberChar(p.s[i]) {
			i++
		}
		if i == 0 {
			p.error("unexpected byte 0x%02x", p.s[0])
			return
		}
		p.cur.value, p.s = p.s[:i], p.s[i:]
	}
	p.offset += len(p.cur.value)
}

func (p *parser) skipWhitespaceAndComments() {
	i := 0
	for i < len(p.s) {
		if isWhitespace(p.s[i]) {
			if p.s[i] == '\n' {
				p.line++
			}
			i++
			continue
		}
		if i+1 < len(p.s) && p.s[i] == '/' && p.s[i+1] == '/' {
			// comment; skip to end of line or input
			for i < len(p.s) && p.s[i] != '\n' {
				i++
			}
			if i < len(p.s) {
				// end of line; keep going
				p.line++
				i++
				continue
			}
			// end of input; fall out of loop
		}
		break
	}
	p.offset += i
	p.s = p.s[i:]
	if len(p.s) == 0 {
		p.done = true
	}
}

func (p *parser) error(format string, a ...interface{}) *parseError {
	pe := &parseError{
		message: fmt.Sprintf(format, a...),
		line:    p.cur.line,
		offset:  p.cur.offset,
	}
	p.cur.err = pe
	p.done = true
	return pe
}

func isWhitespace(c byte) bool {
	// TODO: do better
	return c == ' ' || c == '\n'
}

// Numbers and identifiers are matched by [-+._A-Za-z0-9]
func isIdentOrNumberChar(c byte) bool {
	switch {
	case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z':
		return true
	case '0' <= c && c <= '9':
		return true
	}
	switch c {
	case '-', '+', '.', '_':
		return true
	}
	return false
}
