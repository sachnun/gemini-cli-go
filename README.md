# Gemini CLI to API Proxy

A Go-based proxy server that converts Google's Gemini CLI tool into OpenAI-compatible API endpoints with streaming support, multiple authentication methods, and Docker deployment ready.

Based on [geminicli2api](https://github.com/gzzhongqi/geminicli2api)

## Quick Start

### Docker

Run the pre-built Docker image - recommended for production deployment.

```bash
# Run
docker run -p 8888:8888 \
  -e GEMINI_AUTH_PASSWORD=your_password \
  ghcr.io/sachnun/gemini-cli-go:latest

```

### Local Development

Run directly from source code for development and testing.

```bash
# Install dependencies
go mod tidy

# Run server
go run cmd/server/main.go

# Build binary
go build -o geminicli2api cmd/server/main.go
```

## Environment Variables

Configure authentication and Google Cloud credentials.

### Required
- `GEMINI_AUTH_PASSWORD`: API authentication password

### Optional (choose one)
- `GEMINI_CREDENTIALS`: Google OAuth credentials JSON string
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to credentials file
- `GOOGLE_CLOUD_PROJECT`: Google Cloud project ID

## API Endpoints

Available endpoints for both OpenAI-compatible and native Gemini APIs.

### OpenAI Compatible
- `POST /v1/chat/completions` - Chat completions (streaming & non-streaming)
- `GET /v1/models` - List available models

### Native Gemini
- `POST /v1beta/models/{model}:generateContent` - Generate content
- `POST /v1beta/models/{model}:streamGenerateContent` - Stream content
- `GET /v1beta/models` - List models

## Usage Example

Basic chat completion using curl with OpenAI-compatible endpoint.

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

Multiple authentication methods supported for API access.

- Bearer Token: `Authorization: Bearer YOUR_PASSWORD`
- Basic Auth: `Authorization: Basic base64(username:YOUR_PASSWORD)`
- Query Parameter: `?key=YOUR_PASSWORD`

## License

MIT License - see [LICENSE](LICENSE) file.