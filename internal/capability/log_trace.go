package capability

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"sounds-great-ai/pkg/pack"
)

// TraceResult is the structured output of log_trace.
type TraceResult struct {
	Matches []LogEntry `json:"matches"`
	Summary string     `json:"summary"`
}

// LogEntry is one filtered log line + context.
type LogEntry struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Level   string   `json:"level"` // ERROR/WARN/INFO/UNKNOWN
	Content string   `json:"content"`
	Context []string `json:"context"`
}

const (
	defaultContextLines = 3
	maxTraceMatches     = 100
)

type LogTrace struct{}

func NewLogTrace() *LogTrace { return &LogTrace{} }

func (l *LogTrace) Name() string    { return "log_trace" }
func (l *LogTrace) Version() string { return "v1" }

func (l *LogTrace) Init(ctx context.Context) error { return nil }
func (l *LogTrace) Health() error                  { return nil }
func (l *LogTrace) Close() error                   { return nil }

// Run reads the log file/dir at input.Path, filters lines by input.Query
// (case-insensitive), extracts ±N context lines, recognizes level prefixes.
func (l *LogTrace) Run(ctx context.Context, input *pack.TaskInput) (*pack.TaskOutput, error) {
	path := input.Path
	if path == "" && input.CapabilityConfig != nil {
		if v, ok := input.CapabilityConfig["log_path"]; ok {
			if s, ok := v.(string); ok {
				path = s
			}
		}
	}
	if path == "" {
		return &pack.TaskOutput{Data: map[string]any{"trace": TraceResult{}}}, nil
	}

	var matches []LogEntry
	_ = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(p) != ".log" && p != path {
			// Allow the explicit file path even without .log extension
			return nil
		}
		fileMatches := scanLogFile(p, input.Query, defaultContextLines)
		matches = append(matches, fileMatches...)
		return nil
	})

	if len(matches) > maxTraceMatches {
		matches = matches[:maxTraceMatches]
	}

	return &pack.TaskOutput{
		Data: map[string]any{
			"trace": TraceResult{
				Matches: matches,
				Summary: "",
			},
		},
	}, nil
}

func scanLogFile(path, query string, contextLines int) []LogEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	var matches []LogEntry
	queryLower := strings.ToLower(query)
	for i, line := range allLines {
		if strings.Contains(strings.ToLower(line), queryLower) {
			start := i - contextLines
			if start < 0 {
				start = 0
			}
			end := i + contextLines
			if end >= len(allLines) {
				end = len(allLines) - 1
			}
			ctx := make([]string, 0, end-start)
			for j := start; j <= end; j++ {
				if j == i {
					continue
				}
				ctx = append(ctx, allLines[j])
			}
			matches = append(matches, LogEntry{
				File:    path,
				Line:    i + 1,
				Level:   detectLevel(line),
				Content: line,
				Context: ctx,
			})
		}
	}
	return matches
}

func detectLevel(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "ERROR") || strings.Contains(upper, "ERR"):
		return "ERROR"
	case strings.Contains(upper, "WARN"):
		return "WARN"
	case strings.Contains(upper, "INFO"):
		return "INFO"
	case strings.Contains(upper, "PANIC") || strings.Contains(upper, "FATAL"):
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
