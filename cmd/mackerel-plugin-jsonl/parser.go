package main

import (
	"bytes"
	"errors"
	"log"

	"github.com/buger/jsonparser"
)

var errFlatPathsFound = errors.New("all flat JSON paths found")

func (opt *Opt) jsonParsed(idx int, value []byte, vt jsonparser.ValueType, err error) {
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	if (vt == jsonparser.NotExist) || (vt == jsonparser.Null) {
		return
	}

	err = opt.aggregatorFunctions[idx].appendData(value)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
}

func (opt *Opt) Parse(b []byte) error {
	if opt.filterByte != nil && !bytes.Contains(b, *opt.filterByte) {
		return nil
	}
	if opt.ignoreByte != nil && bytes.Contains(b, *opt.ignoreByte) {
		return nil
	}
	if opt.SkipUntilBracket {
		i := bytes.IndexByte(b, '{')
		if i > 0 {
			b = b[i:]
		}
	}

	if opt.flatPaths {
		opt.parseFlat(b)
	} else {
		jsonparser.EachKey(b, opt.jsonParsed, opt.paths...)
	}
	return nil
}

// parseFlat reads only root-level keys. ObjectEach decodes escaped key names,
// so comparisons here use the same names as paths passed to EachKey.
func (opt *Opt) parseFlat(b []byte) {
	var found uint64
	remaining := len(opt.paths)
	err := jsonparser.ObjectEach(b, func(key, value []byte, valueType jsonparser.ValueType, _ int) error {
		for i, path := range opt.paths {
			bit := uint64(1) << i
			if found&bit != 0 || !bytes.Equal(key, []byte(path[0])) {
				continue
			}
			found |= bit
			remaining--
			opt.jsonParsed(i, value, valueType, nil)
		}
		if remaining == 0 {
			return errFlatPathsFound
		}
		return nil
	})
	if err != nil && err != errFlatPathsFound { //nolint:errorlint
		// Keep EachKey's behavior for non-object and malformed input. Skip values
		// already delivered by ObjectEach so aggregators never count them twice.
		jsonparser.EachKey(b, func(i int, value []byte, valueType jsonparser.ValueType, err error) {
			if i < 0 || found&(uint64(1)<<i) == 0 {
				opt.jsonParsed(i, value, valueType, err)
			}
		}, opt.paths...)
	}
}

func (opt *Opt) Finish(duration float64) {
	opt.duration = duration
}
