package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mackerelio/golib/pluginutil"
	"github.com/monitoring-forge/followparser"
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

func (p *Opt) check() error {
	if len(p.KeyNames) == 0 {
		return fmt.Errorf("specify --key-name <name> --json-path <path> --aggregator <type>")
	}

	if len(p.KeyNames) != len(p.JsonKeys) || len(p.KeyNames) != len(p.Aggregator) {
		return fmt.Errorf("--key-name, --json-path and --aggregator must be specified the same number of times")
	}

	for i := 0; i < len(p.KeyNames); i++ {
		var keys []string
		var modifiers []JsonKeyModifier
		var initializers []JsonKeyInitializer
		var err error
		switch p.Aggregator[i] {
		case "count", "percentile":
			keys, modifiers, initializers, err = parseJsonKeyWithFunc(p.JsonKeys[i])
			if err != nil {
				return fmt.Errorf("invalid json key: %w", err)
			}
			if len(modifiers) > 0 || (len(initializers) > 0) {
				return fmt.Errorf("modifiers and initializers are not supported for %s aggregator", p.Aggregator[i])
			}
		case "group_by", "group_by_with_percentage":
			keys, modifiers, initializers, err = parseJsonKeyWithFunc(p.JsonKeys[i])
			if err != nil {
				return fmt.Errorf("invalid json key: %w", err)
			}
		default:
			return fmt.Errorf("unknown aggregator: %s", p.Aggregator[i])
		}
		af := &AggregatorFunction{
			name:                p.KeyNames[i],
			jsonKey:             keys,
			JsonKeyModifiers:    modifiers,
			JsonKeyInitializers: initializers,
			aggregator:          p.Aggregator[i],
			count:               0,
			groupBy:             map[string]int{},
			percentiles:         []float64{},
		}
		p.aggregatorFunctions = append(p.aggregatorFunctions, af)
	}

	if p.Filter != "" {
		b := []byte(p.Filter)
		p.filterByte = &b
	}
	if p.Ignore != "" {
		b := []byte(p.Ignore)
		p.ignoreByte = &b
	}

	paths := [][]string{}
	for _, af := range p.aggregatorFunctions {
		paths = append(paths, af.jsonKey)
	}
	p.paths = paths
	return nil
}

func (p *Opt) run() (string, error) {
	err := p.check()
	if err != nil {
		return "", err
	}
	parser := NewParser(p)
	fp := &followparser.Parser{
		WorkDir:  pluginutil.PluginWorkDir(),
		Callback: parser,
		Silent:   !p.Verbose,
	}
	if p.LogArchiveDir != "" {
		fp.ArchiveDir = p.LogArchiveDir
	}
	_, err = fp.Parse(
		fmt.Sprintf("%s-mackerel-plugin-jsonl", p.Prefix),
		p.LogFile,
	)
	if err != nil {
		return "", err
	}
	output := p.output()
	return output, nil
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
	for i := 0; i < len(p.aggregatorFunctions); i++ {
		af := p.aggregatorFunctions[i]
		switch af.aggregator {
		case "count":
			if p.duration == 0 {
				// avoid division by zero
				continue
			}
			fmt.Fprintf(&output, "%s.%s\t%f\t%d\n", p.Prefix, af.name, p.calculatePerDuration(af.count), now)
		case "group_by", "group_by_with_percentage":
			if p.duration == 0 {
				// avoid division by zero
				continue
			}
			modifiedMap := map[string]int{}
			modifiedMap = af.applyInitializers(modifiedMap)
			for k, v := range af.groupBy {
				safeKey := strings.ReplaceAll(k, " ", "_")
				safeKey = strings.ReplaceAll(safeKey, ".", "_")
				modifiedKey := af.applyModifiers(safeKey)
				modifiedMap[modifiedKey] += v
			}
			af.groupBy = modifiedMap
			total := 0
			for k, v := range af.groupBy {
				safeKey := strings.ReplaceAll(k, " ", "_")
				safeKey = strings.ReplaceAll(safeKey, ".", "_")
				fmt.Fprintf(&output, "%s.%s.%s\t%f\t%d\n", p.Prefix, af.name, safeKey, p.calculatePerDuration(v), now)
				total += v
			}
			if af.aggregator == "group_by_with_percentage" && total > 0 {
				for k, v := range af.groupBy {
					safeKey := strings.ReplaceAll(k, " ", "_")
					safeKey = strings.ReplaceAll(safeKey, ".", "_")
					percentage := float64(v) / float64(total) * 100
					fmt.Fprintf(&output, "%s.%s_percentage.%s\t%f\t%d\n", p.Prefix, af.name, safeKey, percentage, now)
				}
			}
		case "percentile":
			if len(af.percentiles) == 0 {
				continue
			}
			mean, _ := stats.Mean(af.percentiles)
			fmt.Fprintf(&output, "%s.%s.mean\t%f\t%d\n", p.Prefix, af.name, mean, now)
			ptiles := map[string]float64{
				"90": 90.0,
				"95": 95.0,
				"99": 99.0,
			}
			for name, ptile := range ptiles {
				ptile, err := stats.Percentile(af.percentiles, ptile)
				if err != nil {
					continue
				}
				fmt.Fprintf(&output, "%s.%s.p%s\t%f\t%d\n", p.Prefix, af.name, name, ptile, now)
			}
		}
	}
	return output.String()
}
