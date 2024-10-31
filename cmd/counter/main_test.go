package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shuaibbapputty/word-counter/internal/fetcher"
	"github.com/stretchr/testify/assert"
)

func TestGetInputFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Choice 1",
			input:    "1\n",
			expected: "data/input/1k-endg-urls.txt",
		},
		{
			name:     "Choice 2",
			input:    "2\n",
			expected: "data/input/10k-endg-urls.txt",
		},
		{
			name:     "Choice 3",
			input:    "3\n",
			expected: "data/input/40k-endg-urls.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			oldStdin := os.Stdin
			r, w, _ := os.Pipe()
			os.Stdin = r
			if _, err := w.Write([]byte(tt.input)); err != nil {
				t.Errorf("Failed to write to pipe: %v", err)
			}
			w.Close()

			filename := getInputFilename()
			assert.Equal(t, tt.expected, filename)

			os.Stdin = oldStdin
		})
	}
}

func TestPrintFinalResults(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	startTime := time.Now().Add(-5 * time.Second) // 5 seconds ago
	wordCounts := []map[string]int{
		{"test": 10},
		{"example": 5},
	}
	f := fetcher.NewFetcher()

	printFinalResults(startTime, wordCounts, f)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	if err != nil {
		t.Errorf("Failed to copy from pipe: %v", err)
	}
	output := buf.String()

	var result struct {
		TopWords []map[string]int `json:"top_words"`
		Metrics  struct {
			DurationSeconds float64 `json:"duration_seconds"`
			Processed       int64   `json:"processed"`
			Errors          int64   `json:"errors"`
			RateLimited     int64   `json:"rate_limited"`
		} `json:"metrics"`
	}

	jsonStr := strings.TrimPrefix(output, "\nFinal Results:\n")
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	if len(result.TopWords) != 2 {
		t.Errorf("Expected 2 top words, got %d", len(result.TopWords))
	}
	if result.TopWords[0]["test"] != 10 {
		t.Errorf("Expected count 10 for 'test', got %d", result.TopWords[0]["test"])
	}
	if result.Metrics.DurationSeconds < 4.9 || result.Metrics.DurationSeconds > 5.1 {
		t.Errorf("Expected duration around 5 seconds, got %f", result.Metrics.DurationSeconds)
	}
}
