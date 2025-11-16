package main

import "strings"

type Redirection struct {
	Type     string
	Filename string
}

type ParsedCommand struct {
	Parts        []string
	Redirections []Redirection
}

type parserState struct {
	parts        []string
	current      strings.Builder
	inQuote      bool
	quoteChar    rune
	runes        []rune
	pos          int
	redirections []Redirection
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
	case '>', '<':
		if p.current.Len() == 0 {
			if redirType, hasRedir := p.checkRedirection(char); hasRedir {
				p.handleRedirection(redirType)
			} else {
				p.current.WriteRune(char)
				p.pos++
			}
		} else {
			currentStr := p.current.String()
			if currentStr == "1" && char == '>' {
				p.current.Reset()
				p.current.Grow(16)
				if p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
					p.handleRedirection("1>>")
				} else {
					p.handleRedirection("1>")
				}
			} else if currentStr == "2" && char == '>' {
				p.current.Reset()
				p.current.Grow(16)
				if p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
					p.handleRedirection("2>>")
				} else {
					p.handleRedirection("2>")
				}
			} else {
				p.finishCurrentPart()
				if redirType, hasRedir := p.checkRedirection(char); hasRedir {
					p.handleRedirection(redirType)
				} else {
					p.current.WriteRune(char)
					p.pos++
				}
			}
		}
	case '&':
		if p.current.Len() == 0 && p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
			p.handleRedirection("&>")
		} else {
			p.current.WriteRune(char)
			p.pos++
		}
	case '1':
		if p.current.Len() == 0 && p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
			redirType, hasRedir := p.checkRedirection1()
			if hasRedir {
				p.handleRedirection(redirType)
			} else {
				p.current.WriteRune(char)
				p.pos++
			}
		} else {
			p.current.WriteRune(char)
			p.pos++
		}
	case '2':
		if p.current.Len() == 0 && p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
			redirType, hasRedir := p.checkRedirection2()
			if hasRedir {
				p.handleRedirection(redirType)
			} else {
				p.current.WriteRune(char)
				p.pos++
			}
		} else {
			p.current.WriteRune(char)
			p.pos++
		}
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

func (p *parserState) checkRedirection(char rune) (string, bool) {
	if char == '>' {
		if p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '>' {
			return ">>", true
		}
		if p.pos+1 < len(p.runes) && p.runes[p.pos+1] == '&' {
			return ">&", true
		}
		return ">", true
	}
	if char == '<' {
		return "<", true
	}
	return "", false
}

func (p *parserState) checkRedirection1() (string, bool) {
	if p.pos+1 < len(p.runes) {
		nextChar := p.runes[p.pos+1]
		if nextChar == '>' {
			if p.pos+2 < len(p.runes) && p.runes[p.pos+2] == '>' {
				return "1>>", true
			}
			return "1>", true
		}
	}
	return "", false
}

func (p *parserState) checkRedirection2() (string, bool) {
	if p.pos+1 < len(p.runes) {
		nextChar := p.runes[p.pos+1]
		if nextChar == '>' {
			if p.pos+2 < len(p.runes) && p.runes[p.pos+2] == '>' {
				return "2>>", true
			}
			return "2>", true
		}
	}
	return "", false
}

func (p *parserState) handleRedirection(redirType string) {
	switch redirType {
	case ">>":
		p.pos += 2
	case ">&":
		p.pos += 2
	case "1>>":
		p.pos += 3
	case "1>":
		p.pos += 2
	case "2>>":
		p.pos += 3
	case "2>":
		p.pos += 2
	case "&>":
		p.pos += 2
	case ">", "<":
		p.pos++
	}

	for p.pos < len(p.runes) && (p.runes[p.pos] == ' ' || p.runes[p.pos] == '\t') {
		p.pos++
	}

	for p.pos < len(p.runes) {
		char := p.runes[p.pos]
		if char == ' ' || char == '\t' {
			break
		}
		if char == '\'' || char == '"' || char == '`' {
			p.startQuote(char)
			for p.pos < len(p.runes) {
				if p.runes[p.pos] == p.quoteChar {
					p.endQuote()
					break
				}
				if p.runes[p.pos] == '\\' && p.quoteChar == '"' {
					p.handleQuotedEscape()
					continue
				}
				p.current.WriteRune(p.runes[p.pos])
				p.pos++
			}
		} else if char == '\\' {
			p.handleUnquotedEscape()
		} else {
			p.current.WriteRune(char)
			p.pos++
		}
	}

	filename := p.current.String()
	if filename != "" {
		p.redirections = append(p.redirections, Redirection{
			Type:     redirType,
			Filename: filename,
		})
		p.current.Reset()
		p.current.Grow(16)
	}
}
