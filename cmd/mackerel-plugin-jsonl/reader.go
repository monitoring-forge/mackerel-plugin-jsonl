package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/montanaflynn/stats"
)

type JsonKeyModifier func(string) string
type JsonKeyInitializer func(map[string]int) map[string]int
type AggregatorFunction struct {
	name                string
	jsonKey             []string
	JsonKeyModifiers    []JsonKeyModifier
	JsonKeyInitializers []JsonKeyInitializer
	aggregator          string
	count               int
	groupBy             map[string]int
	percentiles         []float64
}

func (af *AggregatorFunction) applyModifiers(s string) string {
	for _, mod := range af.JsonKeyModifiers {
		s = mod(s)
	}
	return s
}

func (af *AggregatorFunction) applyInitializers(m map[string]int) map[string]int {
	for _, init := range af.JsonKeyInitializers {
		m = init(m)
	}
	return m
}

func (af *AggregatorFunction) appendData(b []byte) error {
	switch af.aggregator {
	case "count":
		af.count++
	case "group_by", "group_by_with_percentage":
		af.groupBy[string(b)]++
	case "percentile":
		floatValue, err := bfloat64(b)
		if err != nil {
			return err
		}
		af.percentiles = append(af.percentiles, floatValue)
	}

	return nil
}

func (p *Opt) validateAndSetup() error {
	if err := p.validateOptions(); err != nil {
		return err
	}

	for i := 0; i < len(p.KeyNames); i++ {
		af, err := p.buildAggregatorFunction(i)
		if err != nil {
			return err
		}
		p.aggregatorFunctions = append(p.aggregatorFunctions, af)
	}

	p.setupFilterBytes()
	p.setupPaths()
	return nil
}

func (p *Opt) validateOptions() error {
	if len(p.KeyNames) == 0 {
		return fmt.Errorf("specify --key-name <name> --json-path <path> --aggregator <type>")
	}
	if len(p.KeyNames) != len(p.JsonKeys) || len(p.KeyNames) != len(p.Aggregator) {
		return fmt.Errorf("--key-name, --json-path and --aggregator must be specified the same number of times")
	}
	return nil
}

func (p *Opt) buildAggregatorFunction(i int) (*AggregatorFunction, error) {
	keys, modifiers, initializers, err := parseJsonKeyWithFunc(p.JsonKeys[i])
	if err != nil {
		return nil, fmt.Errorf("invalid json key: %w", err)
	}

	switch p.Aggregator[i] {
	case "count", "percentile":
		if len(modifiers) > 0 || len(initializers) > 0 {
			return nil, fmt.Errorf("modifiers and initializers are not supported for %s aggregator", p.Aggregator[i])
		}
	case "group_by", "group_by_with_percentage":
		// OK
	default:
		return nil, fmt.Errorf("unknown aggregator: %s", p.Aggregator[i])
	}

	return &AggregatorFunction{
		name:                p.KeyNames[i],
		jsonKey:             keys,
		JsonKeyModifiers:    modifiers,
		JsonKeyInitializers: initializers,
		aggregator:          p.Aggregator[i],
		count:               0,
		groupBy:             map[string]int{},
		percentiles:         []float64{},
	}, nil
}

func (p *Opt) setupFilterBytes() {
	if p.Filter != "" {
		b := []byte(p.Filter)
		p.filterByte = &b
	}
	if p.Ignore != "" {
		b := []byte(p.Ignore)
		p.ignoreByte = &b
	}
}

func (p *Opt) setupPaths() {
	paths := make([][]string, 0, len(p.aggregatorFunctions))
	for _, af := range p.aggregatorFunctions {
		paths = append(paths, af.jsonKey)
	}
	p.paths = paths
}

func (p *Opt) calculatePerDuration(i int) float64 {
	if p.PerSec {
		return float64(i) / p.duration
	}
	return (float64(i) / p.duration) * 60
}

func (p *Opt) output() string {
	now := uint64(time.Now().Unix())
	var output strings.Builder
	for _, af := range p.aggregatorFunctions {
		p.writeAggregatorOutput(&output, af, now)
	}
	return output.String()
}

func (p *Opt) writeAggregatorOutput(output *strings.Builder, af *AggregatorFunction, now uint64) {
	switch af.aggregator {
	case "count":
		p.writeCountOutput(output, af, now)
	case "group_by", "group_by_with_percentage":
		p.writeGroupByOutput(output, af, now)
	case "percentile":
		p.writePercentileOutput(output, af, now)
	}
}

func (p *Opt) writeCountOutput(output *strings.Builder, af *AggregatorFunction, now uint64) {
	if p.duration == 0 {
		// avoid division by zero
		return
	}
	fmt.Fprintf(output, "%s.%s\t%f\t%d\n", p.Prefix, af.name, p.calculatePerDuration(af.count), now)
}

func (p *Opt) writeGroupByOutput(output *strings.Builder, af *AggregatorFunction, now uint64) {
	if p.duration == 0 {
		// avoid division by zero
		return
	}
	af.groupBy = buildModifiedGroupByMap(af)
	total := 0
	for k, v := range af.groupBy {
		safeKey := sanitizeMetricKey(k)
		fmt.Fprintf(output, "%s.%s.%s\t%f\t%d\n", p.Prefix, af.name, safeKey, p.calculatePerDuration(v), now)
		total += v
	}
	if af.aggregator == "group_by_with_percentage" && total > 0 {
		p.writeGroupByPercentageOutput(output, af, total, now)
	}
}

func buildModifiedGroupByMap(af *AggregatorFunction) map[string]int {
	modifiedMap := map[string]int{}
	modifiedMap = af.applyInitializers(modifiedMap)
	for k, v := range af.groupBy {
		safeKey := sanitizeMetricKey(k)
		modifiedKey := af.applyModifiers(safeKey)
		modifiedMap[modifiedKey] += v
	}
	return modifiedMap
}

func (p *Opt) writeGroupByPercentageOutput(output *strings.Builder, af *AggregatorFunction, total int, now uint64) {
	for k, v := range af.groupBy {
		safeKey := sanitizeMetricKey(k)
		percentage := float64(v) / float64(total) * 100
		fmt.Fprintf(output, "%s.%s_percentage.%s\t%f\t%d\n", p.Prefix, af.name, safeKey, percentage, now)
	}
}

func (p *Opt) writePercentileOutput(output *strings.Builder, af *AggregatorFunction, now uint64) {
	if len(af.percentiles) == 0 {
		return
	}
	mean, _ := stats.Mean(af.percentiles)
	fmt.Fprintf(output, "%s.%s.mean\t%f\t%d\n", p.Prefix, af.name, mean, now)
	for name, ptile := range percentileTargets() {
		value, err := stats.Percentile(af.percentiles, ptile)
		if err != nil {
			continue
		}
		fmt.Fprintf(output, "%s.%s.p%s\t%f\t%d\n", p.Prefix, af.name, name, value, now)
	}
}

func percentileTargets() map[string]float64 {
	return map[string]float64{
		"90": 90.0,
		"95": 95.0,
		"99": 99.0,
	}
}

func sanitizeMetricKey(k string) string {
	safeKey := strings.ReplaceAll(k, " ", "_")
	return strings.ReplaceAll(safeKey, ".", "_")
}
