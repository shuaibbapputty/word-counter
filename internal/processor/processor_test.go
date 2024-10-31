package processor

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessValidWordBank(t *testing.T) {
	tests := []struct {
		name     string
		rawWords []string
		want     []string
	}{
		{
			name:     "valid words",
			rawWords: []string{"hello", "world", "test"},
			want:     []string{"hello", "world", "test"},
		},
		{
			name:     "mixed case words",
			rawWords: []string{"Hello", "WORLD", "Test"},
			want:     []string{"hello", "world", "test"},
		},
		{
			name:     "invalid words filtered",
			rawWords: []string{"hi", "hello123", "test!", "valid"},
			want:     []string{"valid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vwb := ProcessValidWordBank(tt.rawWords)
			got := strings.Split(strings.TrimSpace(vwb.GetWords()), "\n")
			sort.Strings(got)
			sort.Strings(tt.want)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessContent(t *testing.T) {
	wordBank := ProcessValidWordBank([]string{"hello", "world", "test"})

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "simple content",
			content: "hello world test",
			want:    []string{"hello", "world", "test"},
		},
		{
			name:    "with punctuation",
			content: "hello, world! test.",
			want:    []string{"hello", "world", "test"},
		},
		{
			name:    "mixed case",
			content: "HELLO World TEST",
			want:    []string{"hello", "world", "test"},
		},
		{
			name:    "invalid words filtered",
			content: "hello invalid123 world",
			want:    []string{"hello", "world"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProcessContent(tt.content, wordBank)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkerPool(t *testing.T) {
	wordBank := ProcessValidWordBank([]string{"hello", "world", "test", "earth"})
	wp := NewWorkerPool(wordBank, -2)
	wp.Start()

	wp.Submit("hello world test")
	wp.Submit("hello test")
	wp.Close()

	totalCounts := make(map[string]int)
	for result := range wp.Results() {
		for word, count := range result {
			totalCounts[word] += count
		}
	}

	assert.Equal(t, 2, totalCounts["hello"])
	assert.Equal(t, 1, totalCounts["world"])
	assert.Equal(t, 2, totalCounts["test"])
}

func TestSafeWordCounter(t *testing.T) {
	counter := NewSafeWordCounter()

	counter.Increment("hello", 2)
	counter.Increment("world", 1)
	counter.Increment("earth", 1)
	counter.Increment("test", 3)

	tests := []struct {
		name string
		topN int
		want []map[string]int
	}{
		{
			name: "top 2",
			topN: 2,
			want: []map[string]int{
				{"test": 3},
				{"hello": 2},
			},
		},
		{
			name: "all words",
			topN: 3,
			want: []map[string]int{
				{"test": 3},
				{"hello": 2},
				{"earth": 1},
			},
		},
		{
			name: "zero words",
			topN: 0,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := counter.GetTopWordCounts(tt.topN)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsAlpha(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{"Hello", false},
		{"hello123", false},
		{"hello!", false},
		{"won't", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isAlpha(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
