package main

import (
	"testing"

	"github.com/monitoring-forge/sampdo"
)

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
	max, err := sorted.Max()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if max != 100 {
		t.Errorf("expected percentiles max 100, got %v", max)
	}
}

func TestParser_Finish(t *testing.T) {
	opt := &Opt{}
	opt.Finish(12.34)
	if opt.duration != 12.34 {
		t.Errorf("expected duration 12.34, got %v", opt.duration)
	}
}
