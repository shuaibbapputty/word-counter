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

2. Build:

   For current platform:

   ```bash
   make build
   ```

   For all platforms:

   ```bash
   make build-all
   ```

   Built binaries will be available in the `bin/` directory:

   - Linux: `counter-linux-amd64`, `counter-linux-arm64`
   - macOS: `counter-darwin-amd64`, `counter-darwin-arm64`
   - Windows: `counter-windows-amd64.exe`

## Usage

1. Using pre-built binary:

   ```bash
   ./bin/counter
   ```

2. Using Make:

   ```bash
   make run
   ```

3. Using Go directly:

   ```bash
   go run cmd/counter/main.go
   ```

4. Follow the prompts to select URL dataset size:

- 1: Process 1,000 urls (can take about 250 - 500 seconds)
- 2: Process 10,000 urls (can take ~ 1.5 hours)
- 3: Process 40,000 urls (can take ~ 6 hours)

## Project Structure

- `cmd/counter/`: Main application entry point
- `internal/`: Internal packages
  - `fetcher/`: URL content fetching with rate limiting
  - `processor/`: Word processing and analysis
- `data/`: Input/output data files
