/*
Package parser parses proto files into gotoc's AST representation.
*/
package parser

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"unicode"

	"github.com/dsymonds/gotoc/internal/ast"
)

func ParseFiles(filenames []string, importPaths []string) (*ast.FileSet, error) {
	// TODO: Use importPaths

	fset := new(ast.FileSet)

	index := make(map[string]int) // filename => index in fset.Files

	for len(filenames) > 0 {
		filename := filenames[0]
		filenames = filenames[1:]
		if _, ok := index[filename]; ok {
			continue // already parsed this one
		}

		f := &ast.File{Name: filename}
		index[filename] = len(fset.Files)
		fset.Files = append(fset.Files, f)

		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}

		p := newParser(string(buf))
		if pe := p.readFile(f); pe != nil {
			return nil, pe
		}
		if p.s != "" {
			return nil, p.errorf("input was not all consumed")
		}

		// enqueue unparsed imports
		for _, imp := range f.Imports {
			if _, ok := index[imp]; !ok {
				filenames = append(filenames, imp)
			}
		}
	}

	if err := resolveSymbols(fset); err != nil {
		return nil, err
	}
	return fset, nil
}

type parseError struct {
	message string
	line    int // 1-based line number
	offset  int // 0-based byte offset from start of input
}

func (pe *parseError) Error() string {
	if pe == nil {
		return "<nil>"
	}
	if pe.line == 1 {
		return fmt.Sprintf("line 1.%d: %v", pe.offset, pe.message)
	}
	return fmt.Sprintf("line %d: %v", pe.line, pe.message)
}

var eof = &parseError{message: "EOF"}

type token struct {
	value        string
	err          *parseError
	line, offset int
	unquoted     string // unquoted version of value
}

func (t *token) astPosition() ast.Position {
	return ast.Position{
		Line:   t.line,
		Offset: t.offset,
	}
}

type parser struct {
	s            string // remaining input
	done         bool
	backed       bool // whether back() was called
	offset, line int
	cur          token

	comments []comment // accumulated during parse
}

type comment struct {
	text         string
	line, offset int
}

func newParser(s string) *parser {
	return &parser{
		s:    s,
		line: 1,
		cur:  token{line: 1},
	}
}

func (p *parser) readFile(f *ast.File) *parseError {
	// Parse top-level things.
	for !p.done {
		tok := p.next()
		if tok.err == eof {
			break
		} else if tok.err != nil {
			return tok.err
		}
		// TODO: enforce ordering? package, imports, remainder
		switch tok.value {
		case "package":
			if f.Package != nil {
				return p.errorf("duplicate package statement")
			}
			tok := p.next()
			if tok.err != nil {
				return tok.err
			}
			f.Package = strings.Split(tok.value, ".") // TODO: validate more
			if err := p.readToken(";"); err != nil {
				return err
			}
		case "syntax":
			if f.Syntax != "" {
				return p.errorf("duplicate syntax statement")
			}
			if err := p.readToken("="); err != nil {
				return err
			}
			tok, err := p.readString()
			if err != nil {
				return err
			}
			switch s := tok.unquoted; s {
			case "proto2", "proto3":
				f.Syntax = s
			default:
				return p.errorf("invalid syntax value %q", s)
			}
			if err := p.readToken(";"); err != nil {
				return err
			}
		case "import":
			tok, err := p.readString()
			if err != nil {
				return err
			}
			f.Imports = append(f.Imports, tok.unquoted)
			if err := p.readToken(";"); err != nil {
				return err
			}
		case "message":
			p.back()
			msg := new(ast.Message)
			f.Messages = append(f.Messages, msg)
			if err := p.readMessage(msg); err != nil {
				return err
			}
			msg.Up = f
		case "enum":
			p.back()
			enum := new(ast.Enum)
			f.Enums = append(f.Enums, enum)
			if err := p.readEnum(enum); err != nil {
				return err
			}
			enum.Up = f
		default:
			return p.errorf("unknown top-level thing %q", tok.value)
		}
	}

	// Handle comments.
	for len(p.comments) > 0 {
		n := 1
		for ; n < len(p.comments); n++ {
			if p.comments[n].line != p.comments[n-1].line+1 {
				break
			}
		}
		c := &ast.Comment{
			Start: ast.Position{
				Line:   p.comments[0].line,
				Offset: p.comments[0].offset,
			},
			End: ast.Position{
				Line:   p.comments[n-1].line,
				Offset: p.comments[n-1].offset,
			},
		}
		for _, comm := range p.comments[:n] {
			c.Text = append(c.Text, comm.text)
		}
		p.comments = p.comments[n:]

		// Strip common whitespace prefix and any whitespace suffix.
		// TODO: this is a bodgy implementation of Longest Common Prefix,
		// and also doesn't do tabs vs. spaces well.
		var prefix string
		for i, line := range c.Text {
			line = strings.TrimRightFunc(line, unicode.IsSpace)
			c.Text[i] = line
			trim := len(line) - len(strings.TrimLeftFunc(line, unicode.IsSpace))
			if i == 0 {
				prefix = line[:trim]
			} else {
				// Check how much of prefix is in common.
				for !strings.HasPrefix(line, prefix) {
					prefix = prefix[:len(prefix)-1]
				}
			}
			if prefix == "" {
				break
			}
		}
		if prefix != "" {
			for i, line := range c.Text {
				c.Text[i] = strings.TrimPrefix(line, prefix)
			}
		}

		f.Comments = append(f.Comments, c)
	}
	// No need to sort comments; they are already in source order.

	return nil
}

func (p *parser) readMessage(msg *ast.Message) *parseError {
	if err := p.readToken("message"); err != nil {
		return err
	}
	msg.Position = p.cur.astPosition()

	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	msg.Name = tok.value // TODO: validate

	if err := p.readToken("{"); err != nil {
		return err
	}

	if err := p.readMessageContents(msg); err != nil {
		return err
	}

	return p.readToken("}")
}

func (p *parser) readMessageContents(msg *ast.Message) *parseError {
	// Parse message fields and other things inside a message.
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		switch tok.value {
		case "message":
			// nested message
			p.back()
			nmsg := new(ast.Message)
			msg.Messages = append(msg.Messages, nmsg)
			if err := p.readMessage(nmsg); err != nil {
				return err
			}
			nmsg.Up = msg
		case "enum":
			// nested enum
			p.back()
			ne := new(ast.Enum)
			msg.Enums = append(msg.Enums, ne)
			if err := p.readEnum(ne); err != nil {
				return err
			}
			ne.Up = msg
		case "extensions":
			// extension range
			p.back()
			r, err := p.readExtensionRange()
			if err != nil {
				return err
			}
			msg.ExtensionRanges = append(msg.ExtensionRanges, r...)
		default:
			// field; this token is required/optional/repeated,
			// a primitive type, or a named type.
			p.back()
			field := new(ast.Field)
			msg.Fields = append(msg.Fields, field)
			if err := p.readField(field); err != nil {
				return err
			}
			field.Up = msg
		case "}":
			// end of message
			p.back()
			return nil
		}
	}
	return p.errorf("unexpected EOF while parsing message")
}

func (p *parser) readField(f *ast.Field) *parseError {
	// look for required/optional/repeated
	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	f.Position = p.cur.astPosition()
	switch tok.value {
	case "required":
		f.Required = true
	case "optional":
		// nothing to do
	case "repeated":
		f.Repeated = true
	default:
		// assume this is a type name
		p.back()
	}

	tok = p.next()
	if tok.err != nil {
		return tok.err
	}
	f.TypeName = tok.value // checked during resolution

	tok = p.next()
	if tok.err != nil {
		return tok.err
	}
	f.Name = tok.value // TODO: validate

	if err := p.readToken("="); err != nil {
		return err
	}

	tag, err := p.readTagNumber(false)
	if err != nil {
		return err
	}
	f.Tag = tag

	if err := p.readToken("["); err == nil {
		// start of options
		// TODO: support more than just default
		if err := p.readToken("default"); err != nil {
			return err
		}
		f.HasDefault = true
		if err := p.readToken("="); err != nil {
			return err
		}
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		// TODO: check type
		switch f.TypeName {
		case "string":
			f.Default = tok.unquoted
		default:
			f.Default = tok.value
		}
		if err := p.readToken("]"); err != nil {
			return err
		}
	} else {
		p.back()
	}

	if err := p.readToken(";"); err != nil {
		return err
	}
	return nil
}

func (p *parser) readExtensionRange() ([][2]int, *parseError) {
	if err := p.readToken("extensions"); err != nil {
		return nil, err
	}

	var rs [][2]int
	for {
		// next token must be a number,
		// followed by a comma, semicolon or "to".
		start, err := p.readTagNumber(false)
		if err != nil {
			return nil, err
		}
		end := start
		tok := p.next()
		if tok.err != nil {
			return nil, err
		}
		if tok.value == "to" {
			end, err = p.readTagNumber(true) // allow "max"
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, p.errorf("bad extension range order: %d > %d", start, end)
			}
			tok = p.next()
			if tok.err != nil {
				return nil, err
			}
		}
		rs = append(rs, [2]int{start, end})
		if tok.value != "," && tok.value != ";" {
			return nil, p.errorf(`got %q, want ",", ";" or "to"`, tok.value)
		}
		if tok.value == ";" {
			break
		}
	}
	return rs, nil
}

func (p *parser) readTagNumber(allowMax bool) (int, *parseError) {
	tok := p.next()
	if tok.err != nil {
		return 0, tok.err
	}
	if allowMax && tok.value == "max" {
		return 1<<29 - 1, nil
	}
	n, err := strconv.ParseInt(tok.value, 10, 32)
	if err != nil {
		return 0, p.errorf("bad field number %q: %v", tok.value, err)
	}
	if n < 1 || n >= 1<<29 {
		return 0, p.errorf("field number %v out of range", n)
	}
	if 19000 <= n && n <= 19999 { // TODO: still relevant?
		return 0, p.errorf("field number %v in reserved range [19000, 19999]", n)
	}
	return int(n), nil
}

func (p *parser) readEnum(enum *ast.Enum) *parseError {
	if err := p.readToken("enum"); err != nil {
		return err
	}
	enum.Position = p.cur.astPosition()

	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	enum.Name = tok.value // TODO: validate

	if err := p.readToken("{"); err != nil {
		return err
	}

	// Parse enum values
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		if tok.value == "}" {
			// end of enum
			return nil
		}
		// TODO: verify tok.value is a valid enum value name.
		ev := new(ast.EnumValue)
		enum.Values = append(enum.Values, ev)
		ev.Position = tok.astPosition()
		ev.Name = tok.value // TODO: validate
		ev.Up = enum

		if err := p.readToken("="); err != nil {
			return err
		}

		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		// TODO: check that tok.value is a valid enum value number.
		num, err := strconv.ParseInt(tok.value, 10, 32)
		if err != nil {
			return p.errorf("bad enum number %q: %v", tok.value, err)
		}
		ev.Number = int32(num) // TODO: validate

		if err := p.readToken(";"); err != nil {
			return err
		}
	}

	return p.errorf("unexpected EOF while parsing enum")
}

func (p *parser) readString() (*token, *parseError) {
	tok := p.next()
	if tok.err != nil {
		return nil, tok.err
	}
	if tok.value[0] != '"' {
		return nil, p.errorf("got %q, want string", tok.value)
	}
	return tok, nil
}

func (p *parser) readToken(want string) *parseError {
	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	if tok.value != want {
		return p.errorf("got %q, want %q", tok.value, want)
	}
	return nil
}

// Back off the parser by one token; may only be done between calls to p.next().
func (p *parser) back() {
	//log.Printf("parser·back(): backed %q [err: %v]", p.cur.value, p.cur.err)
	p.done = false // in case this was the last token
	p.backed = true
	p.cur.err = nil // in case an error was being recovered
}

// Advances the parser and returns the new current token.
func (p *parser) next() *token {
	if p.backed || p.done {
		p.backed = false
	} else {
		p.advance()
		//log.Printf("parser·next(): advanced to %q [err: %v]", p.cur.value, p.cur.err)
		if p.done && p.cur.err == nil {
			p.cur.value = ""
			p.cur.err = eof
		}
	}
	//log.Printf("parser·next(): returning %q [err: %v]", p.cur.value, p.cur.err)
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
	case ';', '{', '}', '=', '[', ']', ',':
		// Single symbol
		p.cur.value, p.s = p.s[:1], p.s[1:]
	case '"', '\'':
		// Quoted string
		i := 1
		for i < len(p.s) && p.s[i] != p.s[0] {
			if p.s[i] == '\\' && i+1 < len(p.s) {
				// skip escaped character
				i++
			}
			i++
		}
		if i >= len(p.s) {
			p.errorf("encountered EOF inside string")
			return
		}
		i++
		p.cur.value, p.s = p.s[:i], p.s[i:]
		// TODO: This doesn't work for single quote strings;
		// quotes will be mangled.
		unq, err := strconv.Unquote(p.cur.value)
		if err != nil {
			p.errorf("invalid quoted string [%s]: %v", p.cur.value, err)
		}
		p.cur.unquoted = unq
	default:
		i := 0
		for i < len(p.s) && isIdentOrNumberChar(p.s[i]) {
			i++
		}
		if i == 0 {
			p.errorf("unexpected byte 0x%02x (%q)", p.s[0], string(p.s[:1]))
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
			si := i + 2
			c := comment{line: p.line, offset: p.offset + i}
			// XXX: set c.text
			// comment; skip to end of line or input
			for i < len(p.s) && p.s[i] != '\n' {
				i++
			}
			c.text = p.s[si:i]
			p.comments = append(p.comments, c)
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

func (p *parser) errorf(format string, a ...interface{}) *parseError {
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
	// TODO: do more accurately
	return unicode.IsSpace(rune(c))
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
