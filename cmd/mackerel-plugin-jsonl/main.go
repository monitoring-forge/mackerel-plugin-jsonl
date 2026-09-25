package main

import (
	"fmt"
	"os"

	"github.com/mackerelio/golib/pluginutil"
	"github.com/monitoring-forge/flagrun"
	"github.com/monitoring-forge/followparser"
)

var version string

type Opt struct {
	Version             bool     `short:"v" long:"version" description:"Show version"`
	Filter              string   `long:"filter" description:"filter string used before check pattern."`
	Ignore              string   `long:"ignore" description:"ignore string used before check pattern."`
	KeyNames            []string `short:"k" long:"key-name" required:"true" description:"Key name for json path"`
	JsonKeys            []string `short:"j" long:"json-key" required:"true" description:"JSON key and modifier functions to extract log message."`
	Aggregator          []string `short:"a" long:"aggregator" required:"true" description:"Aggregator type: count, group_by, group_by_with_percentage, percentile, or percentile(max,min,mean,50,99.9)."` // nolint:staticcheck
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

func (opt *Opt) Run(_ []string) (any, int) {
	fp := &followparser.Parser{
		WorkDir:  pluginutil.PluginWorkDir(),
		Callback: opt,
		Silent:   !opt.Verbose,
	}
	if opt.LogArchiveDir != "" {
		fp.ArchiveDir = opt.LogArchiveDir
	}
	_, err := fp.Parse(
		fmt.Sprintf("%s-mackerel-plugin-jsonl", opt.Prefix),
		opt.LogFile,
	)
	if err != nil {
		return err, flagrun.CRITICAL
	}
	output := opt.output()
	return output, flagrun.OK
}

func main() {
	opt := &Opt{}
	os.Exit(flagrun.Go(opt, flagrun.Version(version), flagrun.Validator(opt.ValidateAndSetup)))
}
