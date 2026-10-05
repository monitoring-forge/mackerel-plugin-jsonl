package main

import (
	"reflect"
	"testing"

	"github.com/monitoring-forge/sampdo"
	"github.com/stretchr/testify/require"
)

func TestParseFlatMatchesEachKey(t *testing.T) {
	tests := []struct {
		name  string
		paths [][]string
		line  string
	}{
		{
			name:  "root keys after nested values",
			paths: [][]string{{"time"}, {"status"}, {"reqtime"}},
			line:  `{"extra":{"status":"nested"},"time":"now","status":"200","reqtime":0.5}`,
		},
		{
			name:  "escaped key and value",
			paths: [][]string{{"status"}, {"a.b"}},
			line:  `{"sta\u0074us":"ok\nnext","a.b":"dotted"}`,
		},
		{
			name:  "duplicate keys use first value",
			paths: [][]string{{"status"}, {"time"}},
			line:  `{"status":"200","status":"500","time":"now"}`,
		},
		{
			name:  "duplicate requested paths",
			paths: [][]string{{"status"}, {"status"}},
			line:  `{"status":"200","status":"500"}`,
		},
		{
			name:  "missing and null values",
			paths: [][]string{{"status"}, {"time"}, {"missing"}},
			line:  `{"status":null,"time":"now"}`,
		},
		{
			name:  "compound values",
			paths: [][]string{{"status"}, {"time"}, {"reqtime"}},
			line:  `{"status":{"code":200},"time":[1,2],"reqtime":true}`,
		},
		{
			name:  "root array uses generic parser",
			paths: [][]string{{"status"}, {"time"}},
			line:  `[{"status":"200"},{"time":"now"}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newOpt := func() *Opt {
				opt := &Opt{}
				for _, path := range tc.paths {
					opt.aggregatorFunctions = append(opt.aggregatorFunctions, &AggregatorFunction{
						jsonKey: path, aggregator: "group_by", groupBy: map[string]int{},
					})
				}
				opt.setupPaths()
				return opt
			}

			fast := newOpt()
			require.True(t, fast.flatPaths, "expected flat path optimization")
			reference := newOpt()
			reference.flatPaths = false
			for _, opt := range []*Opt{fast, reference} {
				err := opt.Parse([]byte(tc.line))
				require.NoError(t, err, "Parse failed")
			}
			for i := range tc.paths {
				if !reflect.DeepEqual(fast.aggregatorFunctions[i].groupBy, reference.aggregatorFunctions[i].groupBy) {
					t.Errorf("path %v: fast=%v, EachKey=%v", tc.paths[i], fast.aggregatorFunctions[i].groupBy, reference.aggregatorFunctions[i].groupBy)
				}
			}
		})
	}
}

func TestSetupPathsSelectsFlatOnlyForSmallRootKeySets(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths [][]string
		want  bool
	}{
		{"one root key", [][]string{{"status"}}, true},
		{"nested key", [][]string{{"status", "code"}}, false},
		{"array key", [][]string{{"items", "[0]"}}, false},
		{"no keys", nil, false},
		{"64 keys", make([][]string, 64), true},
		{"65 keys", make([][]string, 65), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opt := &Opt{}
			for _, path := range tc.paths {
				if path == nil {
					path = []string{"status"}
				}
				opt.aggregatorFunctions = append(opt.aggregatorFunctions, &AggregatorFunction{jsonKey: path})
			}
			opt.setupPaths()
			if opt.flatPaths != tc.want {
				t.Errorf("flatPaths=%v, want %v", opt.flatPaths, tc.want)
			}
		})
	}
}

func TestParser_Parse(t *testing.T) {
	opt := &Opt{
		aggregatorFunctions: []*AggregatorFunction{
			{
				aggregator: "count",
				jsonKey:    []string{"foo"},
				count:      0,
			},
			{
				aggregator: "group_by",
				jsonKey:    []string{"status"},
				groupBy:    map[string]int{},
			},
			{
				aggregator:  "percentile",
				jsonKey:     []string{"ptime"},
				percentiles: sampdo.New(sampdo.WithInitialCapacity(1024)),
			},
		},
		paths: [][]string{{"foo"}, {"status"}, {"ptime"}},
	}

	json := []byte(`{"foo": 1, "status": "ok", "ptime": 100}`)
	err := opt.Parse(json)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if opt.aggregatorFunctions[0].count != 1 {
		t.Errorf("expected count 1, got %v", opt.aggregatorFunctions[0].count)
	}
	if opt.aggregatorFunctions[1].groupBy["ok"] != 1 {
		t.Errorf("expected groupBy ok 1, got %v", opt.aggregatorFunctions[1].groupBy["ok"])
	}
	sorted, err := opt.aggregatorFunctions[2].percentiles.Sorted()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if sorted.Count() != 1 {
		t.Errorf("expected percentiles count 1, got %v", sorted.Count())
	}
	maxPtime, err := sorted.Max()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if maxPtime != 100 {
		t.Errorf("expected percentiles max 100, got %v", maxPtime)
	}
}

func TestParser_Finish(t *testing.T) {
	opt := &Opt{}
	opt.Finish(12.34)
	if opt.duration != 12.34 {
		t.Errorf("expected duration 12.34, got %v", opt.duration)
	}
}
