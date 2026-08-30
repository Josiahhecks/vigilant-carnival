# ⚡ Ultra Fast Roblox User Lookup in Go

A high-performance Roblox User Lookup tool built in **Go** utilizing goroutines for concurrent API fetching and sub-second response times. Includes a modern web interface ready for deployment to **GitHub Pages**.

## Features

- **Sub-Second Lookup**: Concurrently queries Roblox APIs using Go `goroutines` (`sync.WaitGroup`) for user profile details, stats, presence, groups, badges, and avatar renders.
- **Smart Query Parsing**: Automatically detects whether input is a numeric **User ID** or **Username**.
- **In-Memory Cache & Connection Pooling**: Built-in TTL cache and custom HTTP transport configuration (`MaxIdleConns: 100`) for low latency.
- **Modern Web UI**: Responsive dark mode design with instant stat counters, 3 avatar preview perspectives (Headshot, Bust, Full Body), groups list, and badges display.
- **GitHub Pages Ready**: Includes automated GitHub Actions workflow (`.github/workflows/deploy.yml`) to deploy the static website to GitHub Pages with client-side API fallbacks.

## Project Structure

```
├── cmd/
│   └── server/
│       └── main.go           # High-performance Go HTTP server & REST API
├── pkg/
│   └── roblox/
│       ├── client.go         # Core Go Roblox client with goroutines & caching
│       └── client_test.go    # Unit tests
├── public/
│   ├── index.html            # Web interface
│   ├── styles.css            # Dark theme styles
│   └── app.js                # Frontend logic
├── .github/
│   └── workflows/
│       └── deploy.yml        # GitHub Pages automated deployment workflow
├── go.mod                    # Go module file
└── README.md
```

## Running Locally

### 1. Run the Go HTTP Server
```bash
go run cmd/server/main.go
```
Open [http://localhost:8080](http://localhost:8080) in your browser.

### 2. Run Tests
```bash
go test -v ./...
```

## GitHub Pages Deployment

Pushing code to `main` or `master` will trigger the GitHub Actions workflow in `.github/workflows/deploy.yml`, which automatically builds and publishes the `public/` directory to GitHub Pages.
