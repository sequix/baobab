package main

import (
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

const eof = -1

// Token represents a token or text string returned from the scanner.
type Token struct {
	Type Type   // The type of this item.
	Text string // The text of this item.
}

// Type identifies the type of lex items.
//go:generate stringer -type Type
type Type int

const (
	EOF        Type = iota
	Error           // error occurred; value is text of error
	LeftParen       // '('
	RightParen      // ')'
	String          // quoted string (includes quotes)
	Word            // space-separated word
)

func (i Token) String() string {
	switch {
	case i.Type == EOF:
		return "EOF"
	case i.Type == Error:
		return "error: " + i.Text
	case len(i.Text) > 10:
		return fmt.Sprintf("%s: %.10q...", i.Type, i.Text)
	}
	return fmt.Sprintf("%s: %q", i.Type, i.Text)
}

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Scanner) stateFn

type Scanner struct {
	r         io.ByteReader
	done      bool
	token     Token
	buf       []byte // I/O buffer, re-used.
	input     string // the line of text being scanned
	pos       int    // current position in the input
	start     int    // start position of this item
	lastRune  rune   // most recent return from next()
	lastWidth int    // size of that rune
}

// NewScanner creates and returns a new scanner.
func NewScanner(r io.ByteReader) *Scanner {
	l := &Scanner{
		r: r,
	}
	return l
}

// loadLine reads the next line of input and stores it in (appends it to) the input.
// (l.input may have data left over when we are called.)
// It strips carriage returns to make subsequent processing simpler.
func (l *Scanner) loadLine() {
	l.buf = l.buf[:0]
	for {
		c, err := l.r.ReadByte()
		if err != nil {
			l.done = true
			break
		}
		if c != '\r' { // There will never be a \r in l.input.
			l.buf = append(l.buf, c)
		}
		if c == '\n' {
			break
		}
	}
	// Reset to beginning of input buffer if there is nothing pending.
	if l.start == l.pos {
		l.input = string(l.buf)
		l.start = 0
		l.pos = 0
	} else {
		l.input += string(l.buf)
	}
}

// readRune reads the next rune from the input.
func (l *Scanner) readRune() (rune, int) {
	if !l.done && l.pos == len(l.input) {
		l.loadLine()
	}
	if l.pos == len(l.input) {
		return eof, 0
	}
	return utf8.DecodeRuneInString(l.input[l.pos:])
}

// next returns the next rune in the input.
func (l *Scanner) next() rune {
	l.lastRune, l.lastWidth = l.readRune()
	l.pos += l.lastWidth
	return l.lastRune
}

// peek returns but does not consume the next rune in the input.
func (l *Scanner) peek() rune {
	r, _ := l.readRune()
	return r
}

// emit passes an item back to the client.
func (l *Scanner) emit(t Type) stateFn {
	text := l.input[l.start:l.pos]
	l.token = Token{t, text}
	l.start = l.pos
	return nil
}

// errorf returns an error token and empties the input.
func (l *Scanner) errorf(format string, args ...interface{}) stateFn {
	l.token = Token{Error, fmt.Sprintf(format, args...)}
	l.start = 0
	l.pos = 0
	l.input = l.input[:0]
	return nil
}

// backup steps back one rune. Should only be called once per call of next.
func (l *Scanner) backup() {
	if l.lastRune == eof {
		return
	}
	if l.pos == l.start {
		l.errorf("internal error: backup at start of input")
	}
	if l.pos > l.start { // TODO can't happen?
		l.pos -= l.lastWidth
	}
}

// Next returns the next token.
func (l *Scanner) Next() Token {
	l.lastRune = eof
	l.lastWidth = 0
	l.token = Token{EOF, "EOF"}
	state := lexAny
	for {
		state = state(l)
		if state == nil {
			return l.token
		}
	}
}

// lexAny scans non-space items.
func lexAny(l *Scanner) stateFn {
	switch r := l.next(); {
	case r == eof:
		return nil
	case unicode.IsSpace(r):
		return lexSpace(l)
	case r == '/':
		switch nr := l.peek(); {
		case nr == '/':
			l.next()
			return lexCommentLine(l)
		case nr == '*':
			l.next()
			return lexCommentBlock(l)
		default:
			return l.errorf("after '/' unrecognized character %#U", nr)
		}
	case unicode.IsLetter(r) || r == '_':
		return lexKeyword(l)
	case r == '`' || r == '"':
		l.backup() // So lexQuote can read the quote character.
		return lexQuote
	case r == '(':
		return l.emit(LeftParen)
	case r == ')':
		return l.emit(RightParen)
	default:
		return l.errorf("unrecognized character %#U", r)
	}
}

// lexSpace scans a run of space characters.
// One space has already been seen.
func lexSpace(l *Scanner) stateFn {
	for unicode.IsSpace(l.peek()) {
		l.next()
	}
	// Skips over the pending input.
	l.start = l.pos
	return lexAny
}

// lexComment scans a line comment. The comment marker // has been consumed.
func lexCommentLine(l *Scanner) stateFn {
	var r rune
	for {
		r = l.next()
		if r == eof || r == '\n' {
			break
		}
	}
	if r == eof {
		return nil
	}
	l.start = l.pos
	return lexAny
}

// lexComment scans a block comment. The comment marker /* has been consumed.
func lexCommentBlock(l *Scanner) stateFn {
	var r rune
	for {
		r = l.next()
		if r == eof {
			break
		}
		if r == '*' && l.peek() == '/' {
			l.next()
			break
		}
	}
	if r == eof {
		return nil
	}
	return lexAny
}

// lexKeyword scans a keyword like package, import...
func lexKeyword(l *Scanner) stateFn {
	for isWordChar(l.peek()) {
		l.next()
	}
	return l.emit(Word)
}

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

// lexQuote scans a quoted string.
// The next character is the quote.
func lexQuote(l *Scanner) stateFn {
	quote := l.next()
	for {
		switch l.next() {
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case quote:
			return l.emit(String)
		}
	}
}
