package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildOpt(keyNames, jsonKeys, aggregator []string) *Opt {
	return &Opt{
		KeyNames:   keyNames,
		JsonKeys:   jsonKeys,
		Aggregator: aggregator,
	}
}

func TestOpt_validateAndSetup(t *testing.T) {
	// 必須パラメータ不足
	opt := &Opt{}
	assert.Error(t, opt.validateAndSetup(), "expected error for empty params")

	// パラメータ数不一致
	opt = buildOpt([]string{"foo"}, []string{"foo"}, []string{"count", "group_by"})
	assert.Error(t, opt.validateAndSetup(), "expected error for param count mismatch")

	// 不正なaggregator
	opt = buildOpt([]string{"foo"}, []string{"foo"}, []string{"invalid"})
	assert.Error(t, opt.validateAndSetup(), "expected error for invalid aggregator")

	// countでmodifier指定時はエラー
	opt = buildOpt([]string{"foo"}, []string{"foo|tolower"}, []string{"count"})
	assert.Error(t, opt.validateAndSetup(), "expected error for modifier with count")

	// percentileでmodifier指定時はエラー
	opt = buildOpt([]string{"foo"}, []string{"foo|tolower"}, []string{"percentile"})
	assert.Error(t, opt.validateAndSetup(), "expected error for modifier with percentile")

	// group_byでmodifier指定時はOK
	opt = buildOpt([]string{"foo"}, []string{"foo|tolower"}, []string{"group_by"})
	assert.NoError(t, opt.validateAndSetup(), "unexpected error for group_by with modifier")

	// 正常系
	opt = buildOpt([]string{"foo"}, []string{"foo"}, []string{"count"})
	assert.NoError(t, opt.validateAndSetup(), "unexpected error")
}

func TestOpt_calculatePerDuration(t *testing.T) {
	opt := &Opt{duration: 60, PerSec: false}
	if v := opt.calculatePerDuration(120); v != 120 {
		t.Errorf("expected %v, got %v", 120, v)
	}
	opt.PerSec = true
	if v := opt.calculatePerDuration(120); v != 2 {
		t.Errorf("expected %v, got %v", 2, v)
	}
}

func TestOpt_Output(t *testing.T) {
	opt := &Opt{
		Prefix: "test",
		aggregatorFunctions: []*AggregatorFunction{
			{
				aggregator: "count",
				name:       "foo",
				count:      10,
			},
		},
		duration: 10,
	}
	out := opt.output()
	if out == "" {
		t.Errorf("expected output, got empty string")
	}
	if !strings.Contains(out, "test.foo\t60.000000\t") {
		t.Errorf("unexpected output: %s", out)
	}
}
