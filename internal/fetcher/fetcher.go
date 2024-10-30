package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

const (
	requestsPerSecond = 4   // making 4 requests per second
	backoffSecs       = 150 // found that ~150s is a good balance between rate limiting and not waiting too long
	maxRetries        = 3
	retryDelaySec     = 5
	workers           = 10
	resultBuffer      = 100
	idleConnTimeout   = backoffSecs * 2
)

type FetcherConfig struct {
	RequestsPerSecond int
	BackoffDuration   time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	WorkerCount       int
	ResultBuffer      int
}

type Fetcher struct {
	client  *http.Client
	limiter *rate.Limiter
	results chan FetchResult
	metrics *fetcherMetrics
	config  FetcherConfig
	backoff *backoffManager
}

type fetcherMetrics struct {
	processed   atomic.Int64
	errors      atomic.Int64
	rateLimited atomic.Int64
}

type backoffManager struct {
	isActive atomic.Bool
	mutex    sync.Mutex
	signal   chan struct{}
}
type FetchResult struct {
	URL        string
	Content    string
	FetchTime  time.Time
	Error      string
	RetryCount int
}

func getDefaultConfig() FetcherConfig {
	return FetcherConfig{
		RequestsPerSecond: requestsPerSecond,
		BackoffDuration:   backoffSecs * time.Second,
		MaxRetries:        maxRetries,
		RetryDelay:        retryDelaySec * time.Second,
		WorkerCount:       workers,
		ResultBuffer:      resultBuffer,
	}
}

func NewFetcher() *Fetcher {
	config := getDefaultConfig()

	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				IdleConnTimeout: idleConnTimeout * time.Second,
			},
		},
		limiter: rate.NewLimiter(
			rate.Every(time.Second/time.Duration(config.RequestsPerSecond)),
			1,
		),
		results: make(chan FetchResult, config.ResultBuffer),
		metrics: &fetcherMetrics{},
		config:  config,
		backoff: newBackoffManager(),
	}
}

func (f *Fetcher) FetchURLs(ctx context.Context, urls []string) <-chan FetchResult {
	urlPool := make(chan struct{}, f.config.WorkerCount)
	var wg sync.WaitGroup

	go func() {
		defer close(f.results)

		for _, url := range urls {
			if ctx.Err() != nil {
				return
			}

			urlPool <- struct{}{}
			wg.Add(1)

			go func(url string) {
				defer wg.Done()
				defer func() { <-urlPool }()

				f.processURL(ctx, url)
			}(url)
		}

		wg.Wait()
	}()

	return f.results
}

func (f *Fetcher) processURL(ctx context.Context, url string) {
	for attempt := 0; attempt < f.config.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return
		}

		if f.backoff.isActive.Load() {
			select {
			case <-ctx.Done():
				return
			case <-f.backoff.signal:
			}
		}

		if err := f.limiter.Wait(ctx); err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				f.sendResult(url, "", attempt, err.Error())
			}
			return
		}

		content, err := f.fetch(ctx, url)
		if err == nil {
			f.metrics.processed.Add(1)
			select {
			case <-ctx.Done():
				return
			default:
				f.sendResult(url, content, attempt, "")
			}
			return
		}

		if isRateLimit(err) {
			f.metrics.rateLimited.Add(1)
			f.handleRateLimit()
			continue
		}

		if attempt == f.config.MaxRetries-1 {
			f.metrics.errors.Add(1)
			select {
			case <-ctx.Done():
				return
			default:
				f.sendResult(url, "", attempt, err.Error())
			}
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(f.calculateBackoff(attempt)):
		}
	}
}

func (f *Fetcher) fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	return f.handleResponse(resp)
}

func (f *Fetcher) handleRateLimit() {
	if !f.backoff.isActive.Load() {
		f.backoff.mutex.Lock()
		f.backoff.isActive.Store(true)
		f.backoff.signal = make(chan struct{}, 1)
		f.backoff.mutex.Unlock()

		go func() {
			time.Sleep(f.config.BackoffDuration)
			f.backoff.isActive.Store(false)
			close(f.backoff.signal)
		}()
	}
}

func (f *Fetcher) handleResponse(resp *http.Response) (string, error) {
	switch resp.StatusCode {
	case http.StatusOK:
		return f.parseContent(resp)
	case http.StatusTooManyRequests, 999:
		return "", &RateLimitError{
			RetryAfter: f.config.BackoffDuration,
			Message:    fmt.Sprintf("Rate limit exceeded (Status %d)", resp.StatusCode),
		}
	case http.StatusNotFound:
		return "", nil
	default:
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
}

func (f *Fetcher) parseContent(resp *http.Response) (string, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}

	doc.Find(".caas-figure, .caas-img, .t-meta, .caas-carousel, .caas-iframe-wrapper, .twitter-tweet-wrapper").Remove()

	contentBuilder := strings.Builder{}
	selectors := []string{
		"#caas-lead-header-undefined",
		".caas-subheadline",
		".caas-body p",
	}

	doc.Find(strings.Join(selectors, ", ")).Each(func(_ int, s *goquery.Selection) {
		contentBuilder.WriteString(s.Text())
		contentBuilder.WriteByte(' ')
	})

	return strings.Join(strings.Fields(contentBuilder.String()), " "), nil
}

func (f *Fetcher) calculateBackoff(attempt int) time.Duration {
	return f.config.RetryDelay * time.Duration(1<<uint(attempt))
}

func (f *Fetcher) sendResult(url, content string, retryCount int, errorMsg string) {
	result := FetchResult{
		URL:        url,
		Content:    content,
		Error:      errorMsg,
		FetchTime:  time.Now(),
		RetryCount: retryCount,
	}

	select {
	case f.results <- result:
	default:
		return
	}
}

func FetchFromFile(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var urls []string
	for _, line := range strings.Split(string(content), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			urls = append(urls, line)
		}
	}
	return urls, nil
}

func SaveToFile(filePath string, content string) error {
	return os.WriteFile(filePath, []byte(content), 0644)
}

type RateLimitError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s, retry after %v", e.Message, e.RetryAfter)
}

func isRateLimit(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

func (f *Fetcher) GetMetrics() struct {
	Processed   int64
	Errors      int64
	RateLimited int64
} {
	return struct {
		Processed   int64
		Errors      int64
		RateLimited int64
	}{
		Processed:   f.metrics.processed.Load(),
		Errors:      f.metrics.errors.Load(),
		RateLimited: f.metrics.rateLimited.Load(),
	}
}

func newBackoffManager() *backoffManager {
	return &backoffManager{
		signal: make(chan struct{}, 1),
	}
}
