package main

import "strings"

type parserState struct {
	parts     []string
	current   strings.Builder
	inQuote   bool
	quoteChar rune
	runes     []rune
	pos       int
}

func (p *parserState) handleUnquotedChar(char rune) {
	switch char {
	case '\'', '"', '`':
		p.startQuote(char)
	case '\\':
		p.handleUnquotedEscape()
	case ' ', '\t':
		p.finishCurrentPart()
		p.pos++
	default:
		p.current.WriteRune(char)
		p.pos++
	}
}

func (p *parserState) handleQuotedChar(char rune) {
	if char == '\\' && p.quoteChar == '"' {
		p.handleQuotedEscape()
	} else if char == p.quoteChar {
		p.endQuote()
	} else {
		p.current.WriteRune(char)
		p.pos++
	}
}

func (p *parserState) startQuote(quoteChar rune) {
	p.inQuote = true
	p.quoteChar = quoteChar
	p.pos++
}

func (p *parserState) endQuote() {
	p.inQuote = false
	p.pos++
}

func (p *parserState) handleUnquotedEscape() {
	if p.pos+1 < len(p.runes) {
		nextChar := p.runes[p.pos+1]
		p.current.WriteRune(nextChar)
		p.pos += 2
	} else {
		p.current.WriteRune('\\')
		p.pos++
	}
}

func (p *parserState) handleQuotedEscape() {
	if p.pos+1 < len(p.runes) {
		nextChar := p.runes[p.pos+1]
		switch nextChar {
		case '$', '`', '"', '\\':
			p.current.WriteRune(nextChar)
			p.pos += 2
		default:
			p.current.WriteRune('\\')
			p.current.WriteRune(nextChar)
			p.pos += 2
		}
	} else {
		p.current.WriteRune('\\')
		p.pos++
	}
}

func (p *parserState) finishCurrentPart() {
	if p.current.Len() > 0 {
		p.parts = append(p.parts, p.current.String())
		p.current.Reset()
		p.current.Grow(16)
	}
}
