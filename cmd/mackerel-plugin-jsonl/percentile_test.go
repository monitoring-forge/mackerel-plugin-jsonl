package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jessevdk/go-flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPercentileOutput(t *testing.T) {
	for _, tc := range []struct {
		aggregator string
		want       string
	}{
		{"percentile", "mean\t50.000000\np90\t90.000000\np95\t95.000000\np99\t99.000000\n"},
		{`percentile("max","min","mean","50","70",90,99.9)`, "max\t100.000000\nmin\t0.000000\nmean\t50.000000\np50\t50.000000\np70\t70.000000\np90\t90.000000\np99_9\t99.900000\n"},
		{`percentile(0,100,"0.5",'99.99')`, "p0\t0.000000\np100\t100.000000\np0_5\t0.500000\np99_99\t99.990000\n"},
		{`percentile( 50 )`, "p50\t50.000000\n"},
		{`percentile( max, 'min', "mean" )`, "max\t100.000000\nmin\t0.000000\nmean\t50.000000\n"},
	} {
		t.Run(tc.aggregator, func(t *testing.T) {
			opt := &Opt{}
			_, err := flags.NewParser(opt, flags.None).ParseArgs([]string{
				"--key-name", "latency", "--json-key", "latency", "--aggregator", tc.aggregator,
				"--prefix", "test", "--log-file", "test.log",
			})
			require.NoError(t, err)
			require.NoError(t, opt.ValidateAndSetup(nil))
			assert.Empty(t, opt.output())
			for _, value := range []int{100, 0, 50} {
				require.NoError(t, opt.Parse(fmt.Appendf(nil, `{"latency":%d}`, value)))
			}
			var out strings.Builder
			opt.writePercentileOutput(&out, opt.aggregatorFunctions[0], 123)
			var want strings.Builder
			for line := range strings.SplitSeq(strings.TrimSuffix(tc.want, "\n"), "\n") {
				fmt.Fprintf(&want, "test.latency.%s\t123\n", line)
			}
			assert.Equal(t, want.String(), out.String())
		})
	}
}

func TestPercentileInvalidArguments(t *testing.T) {
	for _, aggregator := range []string{
		"percentile()", "percentile( )", `percentile("")`, "percentile(50,)", "percentile(,50)",
		"percentile(50,,90)", "percentile(50,50)", "percentile(median)", "percentile(MAX)", "percentile(-0.1)",
		"percentile(100.1)", "percentile(NaN)", "percentile(Inf)", "percentile(-Inf)",
		"percentile(1e999)", "percentile(50", "percentile50)", `percentile("50)`,
		"percentile(50)extra", "unknown",
	} {
		t.Run(aggregator, func(t *testing.T) {
			opt := buildOpt([]string{"latency"}, []string{"latency"}, []string{aggregator})
			assert.Error(t, opt.ValidateAndSetup(nil))
		})
	}
}

func TestCustomPercentileRejectsModifiers(t *testing.T) {
	for _, key := range []string{"latency|tolower", `latency|have("a")`} {
		opt := buildOpt([]string{"latency"}, []string{key}, []string{"percentile(50)"})
		assert.ErrorContains(t, opt.ValidateAndSetup(nil), "modifiers and initializers are not supported")
	}
}

func TestPercentileIndependentTargets(t *testing.T) {
	opt := buildOpt([]string{"first", "second"}, []string{"a", "b"}, []string{"percentile(min)", "percentile(100)"})
	opt.Prefix = "test"
	require.NoError(t, opt.ValidateAndSetup(nil))
	require.NoError(t, opt.Parse([]byte(`{"a":1,"b":10}`)))
	require.NoError(t, opt.Parse([]byte(`{"a":2,"b":20}`)))
	out := opt.output()
	assert.Contains(t, out, "test.first.min\t1.000000\t")
	assert.Contains(t, out, "test.second.p100\t20.000000\t")
	assert.Equal(t, 2, strings.Count(out, "\n"))
}
