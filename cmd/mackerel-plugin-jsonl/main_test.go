package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/monitoring-forge/followparser"
)

func generateJSONLFile(b testing.TB, dir, filename string, numLines int) error {
	b.Helper()
	filepath := fmt.Sprintf("%s/%s", dir, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	for i := 0; i < numLines; i++ {
		line := fmt.Sprintf(`{"time": "%s", "status": "%d", "reqtime": "%f", "host": "%s", "req": "%s", "method": "%s", "size": "%d", "ua": "%s"}`,
			time.Now().Format(time.RFC3339),
			200+i%5,
			float64(i)/100.0,
			"10.20.30.40",
			"GET /example/path HTTP/1.1",
			"GET",
			941,
			"Mozilla/5.0 (Linux; Android 4.4.2; SO-01F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/73.0.3683.90 Mobile Safari/537.36",
		)
		_, err := file.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}

func resetFollowParserStateFile(b testing.TB, dir, filename, prefix string) error {
	b.Helper()
	stateFilepath := filepath.Join(dir, fmt.Sprintf("%s-mackerel-plugin-jsonl-%d", prefix, os.Geteuid()))
	stateFile, err := os.Create(stateFilepath)
	if err != nil {
		return err
	}
	defer stateFile.Close()

	filepath := fmt.Sprintf("%s/%s", dir, filename)
	stats, err := os.Stat(filepath)
	if err != nil {
		return err
	}
	inode := stats.Sys().(*syscall.Stat_t).Ino
	dev := stats.Sys().(*syscall.Stat_t).Dev
	content := fmt.Sprintf(`{"pos": %d, "time": %f, "inode": %d, "dev": %d}`, 0, float64(time.Now().Unix()-10), inode, dev)
	_, err = stateFile.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}

func initParserForTest(b testing.TB, tmpDir string, numLines int) (*followparser.Parser, *Opt) {
	b.Helper()
	// mackerel-plugin-jsonl --prefix json --log-file json.log -k total.count -j time -a count -k status -j 'status|replace("^(?:([1235])\d{2}|(4)(?:[0-8]\d|9[0-8]))$","${1}${2}xx")|have("2xx","3xx","4xx","499","5xx")' -a group_by_with_percentage -k latency -j reqtime -a percentile
	opt := &Opt{
		KeyNames:   []string{"total.count", "status", "latency"},
		JsonKeys:   []string{"time", `status|replace("^(?:([1235])\d{2}|(4)(?:[0-8]\d|9[0-8]))$","${1}${2}xx")|have("2xx","3xx","4xx","499","5xx")`, "reqtime"},
		Aggregator: []string{"count", "group_by_with_percentage", "percentile"},
		Prefix:     "json",
		LogFile:    "json.log",
	}
	err := opt.validateAndSetup()
	if err != nil {
		b.Fatalf("validateAndSetup failed: %v", err)
	}

	err = generateJSONLFile(b, tmpDir, opt.LogFile, numLines)
	if err != nil {
		b.Fatalf("generateJSONLFile failed: %v", err)
	}

	parser := NewParser(opt)
	fp := &followparser.Parser{
		ArchiveDir: tmpDir,
		WorkDir:    tmpDir,
		Callback:   parser,
		Silent:     true,
	}
	return fp, opt
}

// generate 100k JSONL file and parse benchmark
func BenchmarkMainParse_jsonl(b *testing.B) {
	tmpDir := b.TempDir()
	fp, opt := initParserForTest(b, tmpDir, 100_000)
	posFile := fmt.Sprintf("%s-mackerel-plugin-jsonl", opt.Prefix)
	logFile := filepath.Join(tmpDir, opt.LogFile)

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		err := resetFollowParserStateFile(b, tmpDir, opt.LogFile, opt.Prefix)
		if err != nil {
			b.Fatalf("resetFollowParserStateFile failed: %v", err)
		}
		b.StartTimer()
		parsed, err := fp.Parse(
			posFile,
			logFile,
		)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
		b.StopTimer()
		if parsed == nil {
			b.Fatalf("Parse returned nil parsed data")
		}
		if len(parsed) != 1 {
			b.Fatalf("Parse returned unexpected number of parsed data: got %d, want 1", len(parsed))
		}
		if parsed[0].Rows != 100_000 {
			b.Fatalf("Parse returned unexpected number of rows: got %d, want 100_000", parsed[0].Rows)
		}
		b.StartTimer()
	}
}
