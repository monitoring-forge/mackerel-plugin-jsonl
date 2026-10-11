package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/monitoring-forge/ltsvparser"
	"github.com/monitoring-forge/sampdo"
)

type JsonKeyModifier func(string) string
type JsonKeyNumericModifier func(float64) float64
type JsonKeyInitializer func(map[string]int) map[string]int
type AggregatorFunction struct {
	name                    string
	jsonKey                 []string
	JsonKeyModifiers        []JsonKeyModifier
	JsonKeyNumericModifiers []JsonKeyNumericModifier
	JsonKeyInitializers     []JsonKeyInitializer
	aggregator              string
	count                   int
	sum                     float64
	groupBy                 map[string]int
	percentiles             *sampdo.Sampdo
	percentileTargets       []percentileTarget
}

func (af *AggregatorFunction) applyModifiers(s string) string {
	for _, mod := range af.JsonKeyModifiers {
		s = mod(s)
	}
	return s
}

func (af *AggregatorFunction) applyNumericModifiers(n float64) float64 {
	for _, mod := range af.JsonKeyNumericModifiers {
		n = mod(n)
	}
	return n
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
	case "percentile", "sum":
		floatValue, err := ltsvparser.ParseFloat(b)
		if err != nil {
			return err
		}
		if af.aggregator == "sum" {
			af.sum += floatValue
			return nil
		}
		err = af.percentiles.Append(floatValue)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Opt) ValidateAndSetup(_ []string) error {
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
	keys, modifiers, numericModifiers, initializers, err := parseJsonKeyWithFunc(p.JsonKeys[i])
	if err != nil {
		return nil, fmt.Errorf("invalid json key: %w", err)
	}

	aggregator, targets, err := parseAggregator(p.Aggregator[i])
	if err != nil {
		return nil, err
	}
	switch aggregator {
	case "count", "sum", "percentile":
		if len(modifiers) > 0 || len(initializers) > 0 {
			return nil, fmt.Errorf("modifiers and initializers are not supported for %s aggregator", p.Aggregator[i])
		}
	case "group_by", "group_by_with_percentage":
		// OK
	default:
		return nil, fmt.Errorf("unknown aggregator: %s", p.Aggregator[i])
	}
	var percentileStore *sampdo.Sampdo
	if aggregator == "percentile" {
		percentileStore = sampdo.New(sampdo.WithInitialCapacity(1024))
	}

	return &AggregatorFunction{
		name:                    p.KeyNames[i],
		jsonKey:                 keys,
		JsonKeyModifiers:        modifiers,
		JsonKeyNumericModifiers: numericModifiers,
		JsonKeyInitializers:     initializers,
		aggregator:              aggregator,
		percentileTargets:       targets,
		count:                   0,
		groupBy:                 map[string]int{},
		percentiles:             percentileStore,
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
	// parseFlat uses a uint64 to track paths already found in each line.
	p.flatPaths = len(p.aggregatorFunctions) > 0 && len(p.aggregatorFunctions) <= 64
	for _, af := range p.aggregatorFunctions {
		paths = append(paths, af.jsonKey)
		if len(af.jsonKey) != 1 {
			p.flatPaths = false
		}
	}
	p.paths = paths
}

func (p *Opt) calculatePerDuration(i float64) float64 {
	if p.PerSec {
		return i / p.duration
	}
	return (i / p.duration) * 60
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
	case "sum":
		p.writeSumOutput(output, af, now)
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
	count := af.applyNumericModifiers(float64(af.count))
	fmt.Fprintf(output, "%s.%s\t%f\t%d\n", p.Prefix, af.name, p.calculatePerDuration(count), now)
}

func (p *Opt) writeSumOutput(output *strings.Builder, af *AggregatorFunction, now uint64) {
	if p.duration == 0 {
		// avoid division by zero
		return
	}
	sum := af.applyNumericModifiers(af.sum)
	fmt.Fprintf(output, "%s.%s\t%f\t%d\n", p.Prefix, af.name, p.calculatePerDuration(sum), now)
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
		f := af.applyNumericModifiers(float64(v))
		fmt.Fprintf(output, "%s.%s.%s\t%f\t%d\n", p.Prefix, af.name, safeKey, p.calculatePerDuration(f), now)
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
	sorted, err := af.percentiles.Sorted()
	if err != nil {
		return
	}
	if sorted.Count() == 0 {
		return
	}
	for _, target := range af.percentileTargets {
		value, err := target.calculate(sorted)
		if err != nil {
			continue
		}
		value = af.applyNumericModifiers(value)
		fmt.Fprintf(output, "%s.%s.%s\t%f\t%d\n", p.Prefix, af.name, target.name, value, now)
	}
}

func sanitizeMetricKey(k string) string {
	safeKey := strings.ReplaceAll(k, " ", "_")
	return strings.ReplaceAll(safeKey, ".", "_")
}
