package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-andiamo/splitter"
)

var ErrEmptyJsonKey = errors.New("json key is empty")

// path.to.key | toupper | tolower | trimspace | replace(regex, repl)
func parseJsonKeyWithFunc(s string) ([]string, []JsonKeyModifier, []JsonKeyInitializer, error) {
	emptyJsonModifier := []JsonKeyModifier{}
	emptyJsonInitializer := []JsonKeyInitializer{}
	// , splitter.Parenthesis, splitter.SquareBracket
	sp, _ := splitter.NewSplitter('|', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped, splitter.Parenthesis, splitter.SquareBrackets)
	// do not unescapeQuotes. do split more times
	keys, err := sp.Split(s, splitter.TrimSpaces, splitter.IgnoreEmpties)
	if err != nil {
		return []string{}, emptyJsonModifier, emptyJsonInitializer, err
	}
	if len(keys) == 0 {
		return []string{}, emptyJsonModifier, emptyJsonInitializer, ErrEmptyJsonKey
	}
	jsonKey, err := parseJsonKey(keys[0])
	if err != nil {
		return []string{}, emptyJsonModifier, emptyJsonInitializer, err
	}
	modifiers := []JsonKeyModifier{}
	initializers := []JsonKeyInitializer{}
	for _, fn := range keys[1:] {
		mod, init, err := parseModifier(fn)
		if err != nil {
			return []string{}, emptyJsonModifier, emptyJsonInitializer, err
		}
		if mod != nil {
			modifiers = append(modifiers, mod)
		}
		if init != nil {
			initializers = append(initializers, init)
		}
	}
	return jsonKey, modifiers, initializers, nil
}

func parseModifier(fn string) (JsonKeyModifier, JsonKeyInitializer, error) {
	switch fn {
	case "tolower":
		return func(s string) string { return strings.ToLower(s) }, nil, nil
	case "toupper":
		return func(s string) string { return strings.ToUpper(s) }, nil, nil
	case "trimspace":
		return func(s string) string { return strings.TrimSpace(s) }, nil, nil
	}

	if strings.HasPrefix(fn, "replace(") && strings.HasSuffix(fn, ")") {
		mod, err := parseReplaceModifier(fn)
		return mod, nil, err
	}

	if strings.HasPrefix(fn, "have(") && strings.HasSuffix(fn, ")") {
		init, err := parseHaveInitializer(fn)
		return nil, init, err
	}

	return nil, nil, fmt.Errorf("unknown modifier: %s", fn)
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

// path.to."foo.baz".[0].key
func parseJsonKey(s string) ([]string, error) {
	sp, _ := splitter.NewSplitter('.', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped, splitter.SquareBrackets)
	keys, err := sp.Split(s, splitter.TrimSpaces, splitter.UnescapeQuotes, splitter.IgnoreEmpties)
	if err != nil {
		return []string{}, err
	}
	return keys, nil
}
