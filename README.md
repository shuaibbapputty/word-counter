# Word Counter

A concurrent web essays word counter that fetches articles and analyzes word frequencies with rate limiting and efficient processing.

## Requirements

- Go 1.21 or higher
- Make (optional, for using Makefile commands)

## Installation

Clone and setup:

```bash
 git clone https://github.com/shuaibbapputty/word-counter
 cd word-counter
 go mod download
```

## Usage

1. Using Make:

   ```bash
   make run
   ```

   Or use direct Go command:

   ```bash
   go run cmd/counter/main.go
   ```

2. Follow the prompts to select URL dataset size:

- 1: Process 1,000 urls
- 2: Process 10,000 urls
- 3: Process 40,000 urls

## Project Structure

- `cmd/counter/`: Main application entry point
- `internal/`: Internal packages
  - `fetcher/`: URL content fetching with rate limiting
  - `processor/`: Word processing and analysis
- `data/`: Input/output data files
