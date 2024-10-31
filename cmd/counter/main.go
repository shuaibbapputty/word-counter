package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/shuaibbapputty/word-counter/internal/fetcher"
	"github.com/shuaibbapputty/word-counter/internal/processor"
)

const (
	defaultNumWorkers = 50
	executionTimeout  = 12 * time.Hour
)

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	filename := getInputFilename()

	urls, err := fetcher.FetchFromFile(filename)
	if err != nil {
		log.Fatalf("Failed to load URLs: %v", err)
	}

	startTime := time.Now()
	log.Printf("Program started at: %v", startTime.Format(time.RFC3339))

	bar := progressbar.Default(int64(len(urls)), "Processing URLs")

	ctx, cancel := context.WithTimeout(context.Background(), executionTimeout)
	defer cancel()

	// Get the validated words from the bank of words
	wordBank, err := initializeWordBank()
	if err != nil {
		log.Fatalf("Failed to initialize word bank: %v", err)
	}

	pool := processor.NewWorkerPool(wordBank, defaultNumWorkers)
	pool.Start()

	// initialize the struct to fetch the urls
	f := fetcher.NewFetcher()

	var wg sync.WaitGroup
	wg.Add(2)
	wordCounter := processor.NewSafeWordCounter()

	done := make(chan struct{})
	go func() {
		<-sigChan
		log.Println("\nReceived interrupt signal. Starting graceful shutdown...")
		cancel()
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	// 1. fetch urls
	go func() {
		defer wg.Done()
		defer pool.Close()

		results := f.FetchURLs(ctx, urls)
		for result := range results {
			select {
			case <-ctx.Done():
				log.Println("Context cancelled, stopping URL processing")
				return
			default:
				pool.Submit(result.Content)
				if err := bar.Add(1); err != nil {
					log.Printf("Failed to update progress bar: %v", err)
				}
			}
		}
	}()

	// 2. collect results
	go func() {
		defer wg.Done()

		for wordFrequencies := range pool.Results() {
			select {
			case <-ctx.Done():
				log.Println("Context cancelled, stopping result collection")
				return
			default:
				for word, frequency := range wordFrequencies {
					wordCounter.Increment(word, frequency)
				}
			}
		}
	}()

	<-done

	finalWordCounts := wordCounter.GetTopWordCounts(10) // get the top 10 words
	printFinalResults(startTime, finalWordCounts, f)
}

func getInputFilename() string {
	fmt.Println("Select the number of URLs to process:")
	fmt.Println("1. 1,000 URLs")
	fmt.Println("2. 10,000 URLs")
	fmt.Println("3. 40,000 URLs")

	var choice int
	fmt.Print("Enter your choice (1, 2, or 3): ")
	if _, err := fmt.Scan(&choice); err != nil {
		log.Fatalf("Failed to read choice: %v", err)
	}

	switch choice {
	case 1:
		return "data/input/1k-endg-urls.txt"
	case 2:
		return "data/input/10k-endg-urls.txt"
	case 3:
		return "data/input/40k-endg-urls.txt"
	default:
		log.Fatalf("Invalid choice. Please select 1, 2, or 3")
		return ""
	}
}

func initializeWordBank() (*processor.ValidWordBank, error) {
	rawWords, err := fetcher.FetchFromFile("data/input/words.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to load bank of words: %v", err)
	}

	wordBank := processor.ProcessValidWordBank(rawWords)
	if err := fetcher.SaveToFile("data/output/valid_word_bank.txt", wordBank.GetWords()); err != nil {
		return nil, fmt.Errorf("failed to save word bank to file: %v", err)
	}

	return wordBank, nil
}

func printFinalResults(startTime time.Time, wordCounts []map[string]int, f *fetcher.Fetcher) {
	metrics := f.GetMetrics()
	output := struct {
		TopWords []map[string]int `json:"top_words"`
		Metrics  struct {
			DurationSeconds float64 `json:"duration_seconds"`
			Processed       int64   `json:"processed"`
			Errors          int64   `json:"errors"`
			RateLimited     int64   `json:"rate_limited"`
		} `json:"metrics"`
	}{
		TopWords: wordCounts,
		Metrics: struct {
			DurationSeconds float64 `json:"duration_seconds"`
			Processed       int64   `json:"processed"`
			Errors          int64   `json:"errors"`
			RateLimited     int64   `json:"rate_limited"`
		}{
			DurationSeconds: time.Since(startTime).Seconds(),
			Processed:       metrics.Processed,
			Errors:          metrics.Errors,
			RateLimited:     metrics.RateLimited,
		},
	}

	jsonOutput, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	fmt.Println("\nFinal Results:")
	fmt.Println(string(jsonOutput))
}
