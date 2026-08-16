package main

import (
	"fmt"
	"os"

	"github.com/mackerelio/golib/pluginutil"
	"github.com/monitoring-forge/flagrun"
	"github.com/monitoring-forge/followparser"
)

var version string

const (
	OK = iota
	WARNING
	CRITICAL
	UNKNOWN
)

type Opt struct {
	Version             bool     `short:"v" long:"version" description:"Show version"`
	Filter              string   `long:"filter" description:"filter string used before check pattern."`
	Ignore              string   `long:"ignore" description:"ignore string used before check pattern."`
	KeyNames            []string `short:"k" long:"key-name" required:"true" description:"Key name for json path"`
	JsonKeys            []string `short:"j" long:"json-key" required:"true" description:"JSON key and modifier functions to extract log message."`
	Aggregator          []string `short:"a" long:"aggregator" required:"true" description:"Aggregator type. valid values are count, group_by, group_by_with_percentage, percentile. count is default." choice:"count" choice:"group_by" choice:"group_by_with_percentage" choice:"percentile"`
	SkipUntilBracket    bool     `long:"skip-until-json" description:"skip reading until first { for json log with plain text header"`
	Prefix              string   `long:"prefix" required:"true" description:"Metric key prefix"`
	PerSec              bool     `long:"per-second" description:"calculate per-seconds count. default per minute count"`
	LogFile             string   `short:"l" long:"log-file" description:"Path to log file" required:"true"`
	LogArchiveDir       string   `long:"log-archive-dir" default:"" description:"Path to log archive directory"`
	Verbose             bool     `long:"verbose" description:"display infomational logs"`
	aggregatorFunctions []*AggregatorFunction
	filterByte          *[]byte
	ignoreByte          *[]byte
	paths               [][]string
	duration            float64
}

func (p *Opt) Run(_ []string) (any, int) {
	err := p.validateAndSetup()
	if err != nil {
		return err, flagrun.UNKNOWN
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
		return err, flagrun.CRITICAL
	}
	output := p.output()
	return output, flagrun.OK
}

func main() {
	os.Exit(flagrun.Go(&Opt{}, flagrun.Version(version)))
}
