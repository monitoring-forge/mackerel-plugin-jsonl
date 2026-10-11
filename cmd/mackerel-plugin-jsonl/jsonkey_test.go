package main

import (
	"reflect"
	"testing"
)

func TestParseJsonKeyWithFunc_basic(t *testing.T) {
	key, mods, nummods, inits, err := parseJsonKeyWithFunc("foo.bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(key, []string{"foo", "bar"}) {
		t.Errorf("expected [foo bar], got %v", key)
	}
	if len(inits) != 0 {
		t.Errorf("expected no initializers, got %d", len(inits))
	}
	if len(mods) != 0 {
		t.Errorf("expected no modifiers, got %d", len(mods))
	}
	if len(nummods) != 0 {
		t.Errorf("expected no numeric modifiers, got %d", len(nummods))
	}
}

func TestParseJsonKeyWithFunc_modifiers(t *testing.T) {
	key, mods, nummods, inits, err := parseJsonKeyWithFunc("foo|tolower|toupper|trimspace|have('a','b','c')|multiply(2)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key[0] != "foo" {
		t.Errorf("expected key 'foo', got %v", key)
	}
	if len(inits) != 1 {
		t.Errorf("expected 1 initializer, got %d", len(inits))
	}
	if len(mods) != 3 {
		t.Errorf("expected 3 modifiers, got %d", len(mods))
	}
	if len(nummods) != 1 {
		t.Errorf("expected 1 numeric modifier, got %d", len(nummods))
	}
	// test modifier chain
	v := " FOO "
	for _, m := range mods {
		v = m(v)
	}
	if v != "FOO" {
		t.Errorf("modifier chain failed, got %v", v)
	}
	// test initializer
	m := map[string]int{}
	m = inits[0](m)
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := m[k]; !ok {
			t.Errorf("initializer failed, key %s not found", k)
		}
	}
	// test numeric modifier
	n := 3.0
	for _, m := range nummods {
		n = m(n)
	}
	if n != 6.0 {
		t.Errorf("numeric modifier failed, got %v", n)
	}
}

func TestParseJsonKeyWithFunc_replace(t *testing.T) {
	{
		key, mods, nummods, inits, err := parseJsonKeyWithFunc("foo|replace('o','a')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key[0] != "foo" {
			t.Errorf("expected key 'foo', got %v", key)
		}
		if len(inits) != 0 {
			t.Errorf("expected no initializers, got %d", len(inits))
		}
		if len(mods) != 1 {
			t.Errorf("expected 1 modifier, got %d", len(mods))
		}
		if len(nummods) != 0 {
			t.Errorf("expected no numeric modifiers, got %d", len(nummods))
		}
		v := mods[0]("foo")
		if v != "faa" {
			t.Errorf("replace modifier failed, got %v", v)
		}
	}
	{
		key, mods, nummods, inits, err := parseJsonKeyWithFunc(`foo|replace('[o|f]','a"')`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key[0] != "foo" {
			t.Errorf("expected key 'foo', got %v", key)
		}
		if len(inits) != 0 {
			t.Errorf("expected no initializers, got %d", len(inits))
		}
		if len(mods) != 1 {
			t.Errorf("expected 1 modifier, got %d", len(mods))
		}
		if len(nummods) != 0 {
			t.Errorf("expected no numeric modifiers, got %d", len(nummods))
		}
		v := mods[0]("foo")
		if v != `a"a"a"` {
			t.Errorf("replace modifier failed, got %v", v)
		}
	}
}

func TestParseJsonKeyWithFunc_errors(t *testing.T) {
	testCases := []struct {
		input string
	}{
		{""},
		{"foo|replace('o')"},
		{"foo|replace('(','a')"},
		{"foo|multiply()"},
		{"foo|multiply(a)"},
		{"foo|multiply(1,2)"},
		{"foo|unknownmod"},
	}
	for _, tc := range testCases {
		_, _, _, _, err := parseJsonKeyWithFunc(tc.input)
		t.Logf("err for input %q: %#v", tc.input, err)
		if err == nil {
			t.Errorf("expected error for input %q", tc.input)
		}
	}
}

func TestParseJsonKey(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"foo.bar.baz", []string{"foo", "bar", "baz"}},
		{`foo."bar.baz".qux`, []string{"foo", "bar.baz", "qux"}},
		{`foo.[0].bar`, []string{"foo", "[0]", "bar"}},
		{`foo."bar.baz".[1].qux`, []string{"foo", "bar.baz", "[1]", "qux"}},
		{`"foo\".bar".baz`, []string{`foo".bar`, "baz"}},
		{`"foo\".bar"."b'az"`, []string{`foo".bar`, "b'az"}},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			key, err := parseJsonKey(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(key, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, key)
			}
		})
	}
}

func TestParseJsonKeyWithFunc_cases(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
		funcs    int
	}{
		{"foo.bar.baz | tolower", []string{"foo", "bar", "baz"}, 1},
		{`foo."bar.baz".qux | tolower`, []string{"foo", "bar.baz", "qux"}, 1},
		{`foo.[0].bar | tolower`, []string{"foo", "[0]", "bar"}, 1},
		{`foo."bar.baz".[1].qux | tolower`, []string{"foo", "bar.baz", "[1]", "qux"}, 1},
		{`"foo\".bar".baz | tolower`, []string{`foo".bar`, "baz"}, 1},
		{`"foo\".bar"."b'az" | tolower`, []string{`foo".bar`, "b'az"}, 1},
		{`foo."ba|r".baz | tolower`, []string{"foo", "ba|r", "baz"}, 1},
		{`foo.'ba|r'.baz | tolower`, []string{"foo", "ba|r", "baz"}, 1},
		{`"foo".[ba|r].baz | tolower`, []string{"foo", "[ba|r]", "baz"}, 1},
		{`foo.(ba|r).baz | replace('ba|r','qux') | tolower`, []string{"foo", "(ba|r)", "baz"}, 2},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			key, funcs, nummods, _, err := parseJsonKeyWithFunc(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(key, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, key)
			}
			if len(funcs) != tc.funcs {
				t.Errorf("expected %d modifier, got %d", tc.funcs, len(funcs))
			}
			if len(nummods) != 0 {
				t.Errorf("expected no numeric modifiers, got %d", len(nummods))
			}
		})
	}
}
