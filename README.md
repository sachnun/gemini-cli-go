# Gemini CLI to API Proxy

A Go-based proxy server that converts Google's Gemini CLI tool into OpenAI-compatible API endpoints.

Based on [geminicli2api](https://github.com/gzzhongqi/geminicli2api)

## Features

- OpenAI-compatible chat completions API
- Native Gemini API proxy
- Streaming support
- Multiple authentication methods
- Docker deployment ready
- No external dependencies

## Quick Start

### Docker

```bash
# Build
docker build -t geminicli2api .

# Run
docker run -p 8888:8888 \
  -e GEMINI_AUTH_PASSWORD=your_password \
  -e GEMINI_CREDENTIALS='{"client_id":"...","token":"..."}' \
  geminicli2api
```

### Local Development

```bash
# Install dependencies
go mod tidy

# Run server
go run cmd/server/main.go

# Build binary
go build -o geminicli2api cmd/server/main.go
```

## Environment Variables

### Required
- `GEMINI_AUTH_PASSWORD`: API authentication password

### Optional (choose one)
- `GEMINI_CREDENTIALS`: Google OAuth credentials JSON string
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to credentials file
- `GOOGLE_CLOUD_PROJECT`: Google Cloud project ID

## API Endpoints

### OpenAI Compatible
- `POST /v1/chat/completions` - Chat completions
- `GET /v1/models` - List models

### Native Gemini
- `POST /v1beta/models/{model}:generateContent` - Generate content
- `POST /v1beta/models/{model}:streamGenerateContent` - Stream content
- `GET /v1beta/models` - List models

## Usage Example

```bash
curl -X POST http://localhost:8888/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_password" \
  -d '{
    "model": "gemini-2.5-pro",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Authentication

- Bearer Token: `Authorization: Bearer YOUR_PASSWORD`
- Basic Auth: `Authorization: Basic base64(username:YOUR_PASSWORD)`
- Query Parameter: `?key=YOUR_PASSWORD`

## Supported Models

- `gemini-2.5-pro`
- `gemini-2.5-flash`
- `gemini-1.5-pro`
- `gemini-1.5-flash`

### Model Variants
- `-search` (e.g., `gemini-2.5-pro-search`) - Google Search grounding
- `-nothinking` (e.g., `gemini-2.5-flash-nothinking`) - Minimal reasoning
- `-maxthinking` (e.g., `gemini-2.5-pro-maxthinking`) - Max reasoning budget

## License

MIT License - see [LICENSE](LICENSE) file.