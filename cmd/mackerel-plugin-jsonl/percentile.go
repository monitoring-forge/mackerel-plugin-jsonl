package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/go-andiamo/splitter"
	"github.com/monitoring-forge/sampdo"
)

type percentileTarget struct {
	name  string
	value float64
}

func parseAggregator(s string) (string, []percentileTarget, error) {
	switch s {
	case "count", "group_by", "group_by_with_percentage":
		return s, nil, nil
	case "percentile":
		return s, []percentileTarget{{name: "mean"}, {name: "p90", value: 90}, {name: "p95", value: 95}, {name: "p99", value: 99}}, nil
	}
	if !strings.HasPrefix(s, "percentile(") || !strings.HasSuffix(s, ")") {
		return "", nil, fmt.Errorf("unknown aggregator: %s", s)
	}
	sp, _ := splitter.NewSplitter(',', splitter.DoubleQuotesBackSlashEscaped, splitter.SingleQuotesDoubleEscaped)
	parts, err := sp.Split(s[len("percentile("):len(s)-1], splitter.TrimSpaces, splitter.UnescapeQuotes)
	if err != nil {
		return "", nil, fmt.Errorf("invalid percentile aggregator: %w", err)
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("percentile() requires at least one statistic or percentile")
	}
	targets := make([]percentileTarget, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		target, err := parsePercentileTarget(strings.TrimSpace(part))
		if err != nil {
			return "", nil, err
		}
		if _, exists := seen[target.name]; exists {
			return "", nil, fmt.Errorf("duplicate percentile target: %s", target.name)
		}
		targets = append(targets, target)
		seen[target.name] = struct{}{}
	}
	return "percentile", targets, nil
}

func parsePercentileTarget(s string) (percentileTarget, error) {
	switch s {
	case "max", "min", "mean":
		return percentileTarget{name: s}, nil
	}
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(value) || value < 0 || value > 100 {
		return percentileTarget{}, fmt.Errorf("invalid percentile target %q: expected max, min, mean, or a number from 0 to 100", s)
	}
	name := "p" + strings.ReplaceAll(strconv.FormatFloat(value, 'f', -1, 64), ".", "_")
	return percentileTarget{name: name, value: value}, nil
}

func (target percentileTarget) calculate(sorted *sampdo.Sorted) (float64, error) {
	switch target.name {
	case "max":
		return sorted.Max()
	case "min":
		return sorted.Min()
	case "mean":
		return sorted.Mean()
	default:
		return sorted.Percentile(target.value)
	}
}
