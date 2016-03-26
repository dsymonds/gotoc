/*
Package parser parses proto files into gotoc's AST representation.
*/
package parser

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/dsymonds/gotoc/ast"
)

const debugging = false

func debugf(format string, args ...interface{}) {
	if debugging {
		log.Printf(format, args...)
	}
}

func ParseFiles(filenames []string, importPaths []string) (*ast.FileSet, error) {
	// Force importPaths to have at least one element.
	if len(importPaths) == 0 {
		importPaths = []string{"."}
	}

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

		// Read the first existing file relative to an element of importPaths.
		var buf []byte
		for _, impPath := range importPaths {
			b, err := ioutil.ReadFile(filepath.Join(impPath, filename))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			buf = b
			break
		}
		if buf == nil {
			return nil, fmt.Errorf("file not found: %s", filename)
		}

		p := newParser(filename, string(buf))
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
	message  string
	filename string
	line     int // 1-based line number
	offset   int // 0-based byte offset from start of input
}

func (pe *parseError) Error() string {
	if pe == nil {
		return "<nil>"
	}
	if pe.line == 1 {
		return fmt.Sprintf("%s:1.%d: %v", pe.filename, pe.offset, pe.message)
	}
	return fmt.Sprintf("%s:%d: %v", pe.filename, pe.line, pe.message)
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
	filename     string
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

func newParser(filename, s string) *parser {
	return &parser{
		filename: filename,
		s:        s,
		line:     1,
		cur:      token{line: 1},
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
			var pkg string
			for {
				tok := p.next()
				if tok.err != nil {
					return tok.err
				}
				if tok.value == ";" {
					break
				}
				if tok.value == "." {
					// okay if we already have at least one package component,
					// and didn't just read a dot.
					if pkg == "" || strings.HasSuffix(pkg, ".") {
						return p.errorf(`got ".", want package name`)
					}
				} else {
					// okay if we don't have a package component,
					// or just read a dot.
					if pkg != "" && !strings.HasSuffix(pkg, ".") {
						return p.errorf(`got %q, want "." or ";"`, tok.value)
					}
					// TODO: validate more
				}
				pkg += tok.value
			}
			f.Package = strings.Split(pkg, ".")
		case "option":
			tok := p.next()
			if tok.err != nil {
				return tok.err
			}
			key := tok.value
			if err := p.readToken("="); err != nil {
				return err
			}
			tok = p.next()
			if tok.err != nil {
				return tok.err
			}
			value := tok.value
			if err := p.readToken(";"); err != nil {
				return err
			}
			f.Options = append(f.Options, [2]string{key, value})
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
			if err := p.readToken("public"); err == nil {
				f.PublicImports = append(f.PublicImports, len(f.Imports))
			} else {
				p.back()
			}
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
		case "service":
			p.back()
			srv := new(ast.Service)
			f.Services = append(f.Services, srv)
			if err := p.readService(srv); err != nil {
				return err
			}
			srv.Up = f
		case "extend":
			p.back()
			ext := new(ast.Extension)
			f.Extensions = append(f.Extensions, ext)
			if err := p.readExtension(ext); err != nil {
				return err
			}
			ext.Up = f
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
	var oneof *ast.Oneof // set while inside a oneof
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		switch tok.value {
		case "extend":
			// extension
			p.back()
			ext := new(ast.Extension)
			msg.Extensions = append(msg.Extensions, ext)
			if err := p.readExtension(ext); err != nil {
				return err
			}
			ext.Up = msg
		case "oneof":
			// oneof
			if oneof != nil {
				return p.errorf("nested oneof not permitted")
			}
			oneof = new(ast.Oneof)
			msg.Oneofs = append(msg.Oneofs, oneof)
			oneof.Position = p.cur.astPosition()

			tok := p.next()
			if tok.err != nil {
				return tok.err
			}
			oneof.Name = tok.value // TODO: validate
			oneof.Up = msg

			if err := p.readToken("{"); err != nil {
				return err
			}
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
			field.Oneof = oneof
			field.Up = msg // p.readField uses this
			if err := p.readField(field); err != nil {
				return err
			}
		case "}":
			if oneof != nil {
				// end of oneof
				oneof = nil
				continue
			}
			// end of message
			p.back()
			return nil
		}
	}
	return p.errorf("unexpected EOF while parsing message")
}

func (p *parser) readField(f *ast.Field) *parseError {
	_, inMsg := f.Up.(*ast.Message)

	// TODO: enforce type limitations if f.Oneof != nil

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
	case "map":
		// map < Key , Value >
		if err := p.readToken("<"); err != nil {
			return err
		}
		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		f.KeyTypeName = tok.value // checked during resolution
		if err := p.readToken(","); err != nil {
			return err
		}
		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		f.TypeName = tok.value // checked during resolution
		if err := p.readToken(">"); err != nil {
			return err
		}
		f.Repeated = true // maps are repeated
		goto parseFromFieldName
	default:
		// assume this is a type name
		p.back()
	}

	tok = p.next()
	if tok.err != nil {
		return tok.err
	}
	f.TypeName = tok.value // checked during resolution

parseFromFieldName:
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

	if f.TypeName == "group" && inMsg {
		if err := p.readToken("{"); err != nil {
			return err
		}

		group := &ast.Message{
			// the current parse position is probably good enough
			Position: p.cur.astPosition(),
			Name:     f.Name,
			Group:    true,
			Up:       f.Up,
		}
		if err := p.readMessageContents(group); err != nil {
			return err
		}
		f.TypeName = f.Name
		msg := f.Up.(*ast.Message)
		msg.Messages = append(msg.Messages, group) // ugh
		if err := p.readToken("}"); err != nil {
			return err
		}
		// A semicolon after a group is optional.
		if err := p.readToken(";"); err != nil {
			p.back()
		}
		return nil
	}

	if err := p.readToken("["); err == nil {
		p.back()
		if err := p.readFieldOptions(f); err != nil {
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

func (p *parser) readFieldOptions(f *ast.Field) *parseError {
	if err := p.readToken("["); err != nil {
		return err
	}
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		// TODO: support more options than just default and packed
		switch tok.value {
		case "default":
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
		case "packed":
			f.HasPacked = true
			if err := p.readToken("="); err != nil {
				return err
			}
			packed, err := p.readBool()
			if err != nil {
				return err
			}
			f.Packed = packed
		default:
			return p.errorf(`got %q, want "default" or "packed"`, tok.value)
		}
		// next should be a comma or ]
		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		if tok.value == "," {
			continue
		}
		if tok.value == "]" {
			return nil
		}
		return p.errorf(`got %q, want "," or "]"`, tok.value)
	}
	return p.errorf("unexpected EOF while parsing field options")
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
			// A semicolon after an enum is optional.
			if err := p.readToken(";"); err != nil {
				p.back()
			}
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

func (p *parser) readService(srv *ast.Service) *parseError {
	if err := p.readToken("service"); err != nil {
		return err
	}
	srv.Position = p.cur.astPosition()

	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	srv.Name = tok.value // TODO: validate

	if err := p.readToken("{"); err != nil {
		return err
	}

	// Parse methods
	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		switch tok.value {
		case "}":
			// end of service
			return nil
		case "rpc":
			// handled below
		default:
			return p.errorf(`got %q, want "rpc" or "}"`, tok.value)
		}

		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		mth := new(ast.Method)
		srv.Methods = append(srv.Methods, mth)
		mth.Position = tok.astPosition()
		mth.Name = tok.value // TODO: validate
		mth.Up = srv

		if err := p.readToken("("); err != nil {
			return err
		}

		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		mth.InTypeName = tok.value // TODO: validate
		if err := p.readToken(")"); err != nil {
			return err
		}
		if err := p.readToken("returns"); err != nil {
			return err
		}
		if err := p.readToken("("); err != nil {
			return err
		}
		tok = p.next()
		if tok.err != nil {
			return tok.err
		}
		mth.OutTypeName = tok.value // TODO: validate

		if err := p.readToken(")"); err != nil {
			return err
		}
		if err := p.readToken(";"); err != nil {
			return err
		}
	}

	return p.errorf("unexpected EOF while parsing service")
}

func (p *parser) readExtension(ext *ast.Extension) *parseError {
	if err := p.readToken("extend"); err != nil {
		return err
	}
	ext.Position = p.cur.astPosition()

	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	ext.Extendee = tok.value // checked during resolution

	if err := p.readToken("{"); err != nil {
		return err
	}

	for !p.done {
		tok := p.next()
		if tok.err != nil {
			return tok.err
		}
		if tok.value == "}" {
			// end of extension
			return nil
		}
		p.back()
		field := new(ast.Field)
		ext.Fields = append(ext.Fields, field)
		field.Up = ext // p.readFile uses this
		if err := p.readField(field); err != nil {
			return err
		}
	}
	return p.errorf("unexpected EOF while parsing extension")
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

func (p *parser) readBool() (bool, *parseError) {
	tok := p.next()
	if tok.err != nil {
		return false, tok.err
	}
	// TODO: check which values for bools are valid.
	switch tok.value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, p.errorf(`got %q, want "true" or "false"`, tok.value)
	}
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
	debugf("parser·back(): backed %q [err: %v]", p.cur.value, p.cur.err)
	p.done = false // in case this was the last token
	p.backed = true
	// In case an error was being recovered, ignore any error.
	// Don't do this for EOF, though, since we know that's what
	// we'll return next.
	if p.cur.err != eof {
		p.cur.err = nil // in case an error was being recovered
	}
}

// Advances the parser and returns the new current token.
func (p *parser) next() *token {
	if p.backed || p.done {
		p.backed = false
	} else {
		p.advance()
		debugf("parser·next(): advanced to %q [err: %v]", p.cur.value, p.cur.err)
		if p.done && p.cur.err == nil {
			p.cur.value = ""
			p.cur.err = eof
		}
	}
	debugf("parser·next(): returning %q [err: %v]", p.cur.value, p.cur.err)
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
	case ';', '{', '}', '=', '[', ']', ',', '<', '>', '(', ')':
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
		message:  fmt.Sprintf(format, a...),
		filename: p.filename,
		line:     p.cur.line,
		offset:   p.cur.offset,
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
