# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based proxy server that converts Google's Gemini CLI tool into both OpenAI-compatible and native Gemini API endpoints. It allows developers to use Google's free Gemini API quota through familiar OpenAI API interfaces.

## Architecture

The application follows a modular package structure:

- `cmd/`: Entry points for two deployment modes (standard server on port 8888, Hugging Face version on port 7860)
- `pkg/auth/`: OAuth2 authentication and credential management with automatic token refresh
- `pkg/routes/`: HTTP route handlers for both OpenAI and Gemini API endpoints
- `pkg/google/`: Google API client integration and streaming support
- `pkg/config/`: Model definitions and configuration management
- `pkg/models/`: Data models for OpenAI and Gemini formats
- `pkg/transformers/`: Request/response format conversion between OpenAI and Gemini

## Development Commands

```bash
# Install dependencies
go mod tidy

# Run development server (port 8888)
go run cmd/server/main.go

# Run Hugging Face compatible server (port 7860)
go run cmd/hf/main.go

# Build binary
go build -o geminicli2api cmd/server/main.go

# Run tests
go test ./...

# Run specific package tests
go test ./pkg/auth
```

## Configuration

Required environment variables:
- `GEMINI_AUTH_PASSWORD`: Password for API access authentication

Optional credential sources (choose one):
- `GEMINI_CREDENTIALS`: JSON string with OAuth credentials
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to credentials file
- `GOOGLE_CLOUD_PROJECT`: Google Cloud project ID

## Key Patterns

1. **Dual Authentication**: Supports both OpenAI-style (Bearer token) and Google-style authentication
2. **Streaming Protocol**: Uses Server-Sent Events (SSE) for streaming responses while maintaining OpenAI format compatibility
3. **Model Variants**: Automatically generates search and thinking variants for base models (e.g., `gemini-2.5-pro-search`, `gemini-2.5-pro-maxthinking`)
4. **OAuth2 Flow**: Complete OAuth2 implementation with refresh token management and credential persistence
5. **Middleware Architecture**: Authentication handled via Gin middleware for both API types

## API Endpoints

OpenAI-compatible:
- `POST /v1/chat/completions` - Chat completions (streaming & non-streaming)
- `GET /v1/models` - List available models

Native Gemini:
- `GET /v1beta/models` - List Gemini models
- `POST /v1beta/models/{model}/generateContent` - Generate content
- `POST /v1beta/models/{model}/streamGenerateContent` - Stream content

## Docker Deployment

```bash
# Build image
docker build -t geminicli2api .

# Run on port 8888 (compatibility mode)
docker run -p 8888:8888 -e GEMINI_AUTH_PASSWORD=your_password geminicli2api

# Run on port 7860 (Hugging Face Spaces)
docker run -p 7860:7860 -e GEMINI_AUTH_PASSWORD=your_password -e PORT=7860 geminicli2api
```

## Dependencies

- Gin web framework for HTTP routing and middleware
- golang.org/x/oauth2 for OAuth2 authentication
- google.golang.org/api for Gemini API integration
- godotenv for environment variable management
- gin-contrib/cors for cross-origin requests