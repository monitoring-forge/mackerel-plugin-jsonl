package main

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-andiamo/splitter"
)

var ErrEmptyJsonKey = errors.New("json key is empty")

// path.to.key | toupper | tolower | trimspace | replace(regex, repl) | have("foo","bar","baz") | multiply(factor)
func parseJsonKeyWithFunc(s string) ([]string, []JsonKeyModifier, []JsonKeyNumericModifier, []JsonKeyInitializer, error) {
	emptyJsonModifier := []JsonKeyModifier{}
	emptyJsonNumericModifier := []JsonKeyNumericModifier{}
	emptyJsonInitializer := []JsonKeyInitializer{}
	// , splitter.Parenthesis, splitter.SquareBracket
	sp, _ := splitter.NewSplitter('|', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped, splitter.Parenthesis, splitter.SquareBrackets)
	// do not unescapeQuotes. do split more times
	keys, err := sp.Split(s, splitter.TrimSpaces, splitter.IgnoreEmpties)
	if err != nil {
		return []string{}, emptyJsonModifier, emptyJsonNumericModifier, emptyJsonInitializer, err
	}
	if len(keys) == 0 {
		return []string{}, emptyJsonModifier, emptyJsonNumericModifier, emptyJsonInitializer, ErrEmptyJsonKey
	}
	jsonKey, err := parseJsonKey(keys[0])
	if err != nil {
		return []string{}, emptyJsonModifier, emptyJsonNumericModifier, emptyJsonInitializer, err
	}
	modifiers := []JsonKeyModifier{}
	numericModifiers := []JsonKeyNumericModifier{}
	initializers := []JsonKeyInitializer{}
	for _, fn := range keys[1:] {
		mod, numMod, init, err := parseModifier(fn)
		if err != nil {
			return []string{}, emptyJsonModifier, emptyJsonNumericModifier, emptyJsonInitializer, err
		}
		if mod != nil {
			modifiers = append(modifiers, mod)
		}
		if numMod != nil {
			numericModifiers = append(numericModifiers, numMod)
		}
		if init != nil {
			initializers = append(initializers, init)
		}
	}
	return jsonKey, modifiers, numericModifiers, initializers, nil
}

func parseModifier(fn string) (JsonKeyModifier, JsonKeyNumericModifier, JsonKeyInitializer, error) {
	switch fn {
	case "tolower":
		return func(s string) string { return strings.ToLower(s) }, nil, nil, nil
	case "toupper":
		return func(s string) string { return strings.ToUpper(s) }, nil, nil, nil
	case "trimspace":
		return func(s string) string { return strings.TrimSpace(s) }, nil, nil, nil
	}

	if strings.HasPrefix(fn, "replace(") && strings.HasSuffix(fn, ")") {
		mod, err := parseReplaceModifier(fn)
		return mod, nil, nil, err
	}

	if strings.HasPrefix(fn, "have(") && strings.HasSuffix(fn, ")") {
		init, err := parseHaveInitializer(fn)
		return nil, nil, init, err
	}

	if strings.HasPrefix(fn, "multiply(") && strings.HasSuffix(fn, ")") {
		mod, err := parseMultiplyModifier(fn)
		return nil, mod, nil, err
	}

	return nil, nil, nil, fmt.Errorf("unknown modifier: %s", fn)
}

func parseReplaceModifier(fn string) (JsonKeyModifier, error) {
	inner := fn[8 : len(fn)-1]
	// replace("pattern","repl")
	s, _ := splitter.NewSplitter(',', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped)
	// must unescapeQuotes
	parts, err := s.Split(inner, splitter.TrimSpaces, splitter.UnescapeQuotes, splitter.IgnoreEmpties)
	if err != nil {
		return nil, err
	}
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid replace() format: %s", fn)
	}
	pattern := parts[0]
	reg, err := regexp.Compile(pattern) // validate regexp
	if err != nil {
		return nil, fmt.Errorf("invalid regexp: %w in %s", err, fn)
	}
	repl := parts[1]
	return func(s string) string { return reg.ReplaceAllString(s, repl) }, nil
}

func parseHaveInitializer(fn string) (JsonKeyInitializer, error) {
	// have("foo","bar","baz")
	inner := fn[5 : len(fn)-1]
	s, _ := splitter.NewSplitter(',', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped)
	parts, err := s.Split(inner, splitter.TrimSpaces, splitter.UnescapeQuotes, splitter.IgnoreEmpties)
	if err != nil {
		return nil, err
	}
	return func(m map[string]int) map[string]int {
		for _, p := range parts {
			m[p] = 0
		}
		return m
	}, nil
}

func parseMultiplyModifier(fn string) (JsonKeyNumericModifier, error) {
	// multiply("1000")
	inner := fn[9 : len(fn)-1]
	s, _ := splitter.NewSplitter(',', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped)
	parts, err := s.Split(inner, splitter.TrimSpaces, splitter.UnescapeQuotes, splitter.IgnoreEmpties)
	if err != nil {
		return nil, err
	}
	if len(parts) != 1 {
		return nil, fmt.Errorf("invalid multiply() format: %s", fn)
	}
	factor, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid multiply factor: %w in %s", err, fn)
	}
	// check if factor is finite or not NaN
	if math.IsNaN(factor) || math.IsInf(factor, 0) {
		return nil, fmt.Errorf("multiply factor must be finite: %s", fn)
	}
	return func(n float64) float64 { return n * factor }, nil
}

// path.to."foo.baz".[0].key
func parseJsonKey(s string) ([]string, error) {
	sp, _ := splitter.NewSplitter('.', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped, splitter.SquareBrackets)
	keys, err := sp.Split(s, splitter.TrimSpaces, splitter.UnescapeQuotes, splitter.IgnoreEmpties)
	if err != nil {
		return []string{}, err
	}
	return keys, nil
}
