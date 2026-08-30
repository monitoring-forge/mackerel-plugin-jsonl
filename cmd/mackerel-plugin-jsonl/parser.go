package main

import (
	"bytes"
	"log"

	"github.com/buger/jsonparser"
)

type Parser struct {
	opt *Opt
}

func NewParser(opt *Opt) *Parser {
	return &Parser{
		opt: opt,
	}
}

func (p *Parser) jsonParsed(idx int, value []byte, vt jsonparser.ValueType, err error) {
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	if (vt == jsonparser.NotExist) || (vt == jsonparser.Null) {
		return
	}

	err = p.opt.aggregatorFunctions[idx].appendData(value)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
}

func (p *Parser) Parse(b []byte) error {
	if p.opt.filterByte != nil && !bytes.Contains(b, *p.opt.filterByte) {
		return nil
	}
	if p.opt.ignoreByte != nil && bytes.Contains(b, *p.opt.ignoreByte) {
		return nil
	}
	if p.opt.SkipUntilBracket {
		i := bytes.IndexByte(b, '{')
		if i > 0 {
			b = b[i:]
		}
	}

	jsonparser.EachKey(b, p.jsonParsed, p.opt.paths...)
	return nil
}

func (p *Parser) Finish(duration float64) {
	p.opt.duration = duration
}
