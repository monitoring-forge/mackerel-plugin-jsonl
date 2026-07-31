package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jessevdk/go-flags"
)

var version string
var commit string

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

func main() {
	os.Exit(_main())
}

func _main() int {
	opt := &Opt{}
	psr := flags.NewParser(opt, flags.HelpFlag|flags.PassDoubleDash)
	_, err := psr.Parse()
	if opt.Version {
		if commit == "" {
			commit = "dev"
		}
		fmt.Printf(
			"%s-%s\n%s/%s, %s, %s\n",
			filepath.Base(os.Args[0]),
			version,
			runtime.GOOS,
			runtime.GOARCH,
			runtime.Version(),
			commit)
		return OK
	} else if flags.WroteHelp(err) {
		fmt.Fprintf(os.Stdout, "%v\n", err)
		return OK
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return UNKNOWN
	}

	output, err := opt.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return CRITICAL
	}
	fmt.Print(output)

	return OK
}
