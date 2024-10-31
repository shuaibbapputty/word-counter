package processor

import (
	"sort"
	"strings"
	"sync"
)

type ValidWordBank struct {
	words map[string]struct{}
}

func ProcessValidWordBank(rawWords []string) *ValidWordBank {
	vwb := &ValidWordBank{
		words: make(map[string]struct{}),
	}

	for _, word := range rawWords {
		word = strings.ToLower(word)
		if len(word) >= 3 && isAlpha(word) {
			vwb.words[word] = struct{}{}
		}
	}

	return vwb
}

func (vwb *ValidWordBank) IsValid(word string) bool {
	_, exists := vwb.words[word]
	return exists
}

func ProcessContent(content string, wordBank *ValidWordBank) []string {
	words := strings.Fields(content)
	validWords := make([]string, 0, len(words))
	buf := make([]byte, 0, 32)

	for _, word := range words {
		buf = buf[:0]
		for i := 0; i < len(word); i++ {
			c := word[i]
			if c >= 'A' && c <= 'Z' {
				buf = append(buf, c+32) // to lowercase
			} else if c >= 'a' && c <= 'z' {
				buf = append(buf, c)
			}
		}

		if len(buf) >= 3 && wordBank.IsValid(string(buf)) {
			validWords = append(validWords, string(buf))
		}
	}
	return validWords
}

func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

type WorkerPool struct {
	wordBank   *ValidWordBank
	numWorkers int
	jobs       chan string
	results    chan map[string]int
	wg         *sync.WaitGroup
}

func NewWorkerPool(wordBank *ValidWordBank, numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 1
	}

	bufferSize := numWorkers * 2
	return &WorkerPool{
		wordBank:   wordBank,
		numWorkers: numWorkers,
		jobs:       make(chan string, bufferSize),
		results:    make(chan map[string]int, bufferSize),
		wg:         &sync.WaitGroup{},
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.numWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()

	for content := range wp.jobs {
		wordCounts := make(map[string]int)
		processedWords := ProcessContent(content, wp.wordBank)

		for _, word := range processedWords {
			wordCounts[word]++
		}

		wp.results <- wordCounts
	}
}

func (wp *WorkerPool) Submit(content string) {
	wp.jobs <- content
}

func (wp *WorkerPool) Close() {
	close(wp.jobs)
	wp.wg.Wait()
	close(wp.results)
}

func (p *WorkerPool) Results() <-chan map[string]int {
	return p.results
}

func (p *ValidWordBank) GetWords() string {
	words := make([]string, 0, len(p.words))
	for word := range p.words {
		words = append(words, word)
	}
	return strings.Join(words, "\n")
}

type SafeWordCounter struct {
	mu     sync.RWMutex
	counts map[string]int
}

func NewSafeWordCounter() *SafeWordCounter {
	return &SafeWordCounter{
		counts: make(map[string]int),
	}
}

func (c *SafeWordCounter) Increment(word string, count int) {
	c.mu.Lock()
	c.counts[word] += count
	c.mu.Unlock()
}

func (c *SafeWordCounter) GetTopWordCounts(topN int) []map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if topN <= 0 {
		return nil
	}

	wcList := make([]struct {
		word  string
		count int
	}, 0, len(c.counts))

	for word, count := range c.counts {
		wcList = append(wcList, struct {
			word  string
			count int
		}{word, count})
	}

	sort.Slice(wcList, func(i, j int) bool {
		if wcList[i].count == wcList[j].count {
			return wcList[i].word < wcList[j].word
		}
		return wcList[i].count > wcList[j].count
	})

	resultLen := min(topN, len(wcList))
	topWords := make([]map[string]int, resultLen)
	for i := 0; i < resultLen; i++ {
		topWords[i] = map[string]int{wcList[i].word: wcList[i].count}
	}

	return topWords
}
