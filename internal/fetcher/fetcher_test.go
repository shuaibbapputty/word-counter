package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFetcher(t *testing.T) {
	f := NewFetcher()
	assert.NotNil(t, f)
	assert.NotNil(t, f.client)
	assert.NotNil(t, f.limiter)
	assert.NotNil(t, f.results)
	assert.NotNil(t, f.metrics)
	assert.NotNil(t, f.backoff)
}

func TestFetchURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`<html><body>
			<div id="caas-lead-header-undefined">Header</div>
			<div class="caas-body"><p>Test content</p></div>
		</body></html>`))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	f := NewFetcher()
	ctx := context.Background()
	urls := []string{server.URL}

	results := f.FetchURLs(ctx, urls)
	result := <-results

	assert.Empty(t, result.Error)
	assert.Contains(t, result.Content, "Header Test content")
}

func TestRateLimitHandling(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("<html><body><p class='caas-subheadline'>Success</p></body></html>"))
		if err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	f := NewFetcher()
	f.config.BackoffDuration = 100 * time.Millisecond // Shorter backoff for testing
	ctx := context.Background()

	results := f.FetchURLs(ctx, []string{server.URL})
	result := <-results

	assert.Empty(t, result.Error)
	assert.Contains(t, result.Content, "Success")
}

func TestFetchFromFile(t *testing.T) {
	content := "http://example.com/1\nhttp://example.com/2\n"
	tmpfile, err := os.CreateTemp("", "urls-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)
	tmpfile.Close()

	urls, err := FetchFromFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Len(t, urls, 2)
	assert.Equal(t, "http://example.com/1", urls[0])
	assert.Equal(t, "http://example.com/2", urls[1])
}

func TestSaveToFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "content-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	content := "test content"
	err = SaveToFile(tmpfile.Name(), content)
	assert.NoError(t, err)

	saved, err := os.ReadFile(tmpfile.Name())
	assert.NoError(t, err)
	assert.Equal(t, content, string(saved))
}

func TestGetMetrics(t *testing.T) {
	f := NewFetcher()
	f.metrics.processed.Add(1)
	f.metrics.errors.Add(2)
	f.metrics.rateLimited.Add(3)

	metrics := f.GetMetrics()
	assert.Equal(t, int64(1), metrics.Processed)
	assert.Equal(t, int64(2), metrics.Errors)
	assert.Equal(t, int64(3), metrics.RateLimited)
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantError  bool
	}{
		{"Success", http.StatusOK, false},
		{"NotFound", http.StatusNotFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_, err := w.Write([]byte("<html><body><p>Content</p></body></html>"))
					if err != nil {
						t.Errorf("failed to write response: %v", err)
					}
				}
			}))
			defer server.Close()

			f := NewFetcher()
			ctx := context.Background()

			results := f.FetchURLs(ctx, []string{server.URL})
			result := <-results
			if tt.wantError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.Empty(t, result.Error)
			}
		})
	}
}
