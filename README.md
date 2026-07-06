# CommandCode Bridge

[Türkçe](README.tr.md)

CommandCode Bridge is a Go reverse proxy that exposes OpenAI-compatible and Anthropic-compatible HTTP endpoints for your CommandCode Go subscription.

It accepts local client requests, converts OpenAI or Anthropic payloads into the CommandCode upstream request shape, forwards them to CommandCode, and translates upstream NDJSON responses back into OpenAI or Anthropic response formats.

## Features

- OpenAI-compatible `POST /v1/chat/completions` endpoint.
- Anthropic-compatible `POST /v1/messages` endpoint.
- OpenAI-compatible `GET /v1/models` endpoint backed by the Provider API model list.
- Streaming and non-streaming response handling.
- Tool calling support for OpenAI and Anthropic responses.
- Anthropic URL and base64 image source conversion to OpenAI `image_url` blocks.
- Per-key session management with request header session reuse.
- Machine fingerprint and CLI compatibility headers for upstream requests.
- Optional local proxy authentication with `proxy_token`.
- Optional fallback upstream credential through `cc_apiKey`.

## Requirements

- Go `1.26.4` or newer.
- Docker and Docker Compose for containerized deployment.
- A CommandCode API key in `user_...` format for upstream API access.

## Setup

1. Install Node.js and npm if they are not already installed.

2. Install the Command Code CLI:

   ```bash
   npm i -g command-code@latest
   ```

3. Log in with the Command Code CLI:

   ```bash
   cmd login
   ```

4. Copy the example config:

   ```bash
   git clone https://github.com/KilimcininKorOglu/CommandCodeBridge
   cd CommandCodeBridge/
   cp data/config.example.json data/config.json
   ```

5. Copy the `apiKey` value from `~/.commandcode/auth.json` into `cc_apiKey` in `data/config.json`. Keep `data/config.json` private.

6. Change `proxy_token` in `data/config.json` to a hard-to-guess local proxy token. Clients must use this token when calling the proxy.

7. Set `projectSlug` if you want a fixed upstream project slug. Leave it empty to use the session-derived fake slug.

8. Keep logs under `data/logs/`:

   ```json
   {
     "logFile": "data/logs/proxy.log"
   }
   ```

9. Build and start with Docker Compose:

   ```bash
   docker compose up -d --build
   ```

10. Verify the service:

    ```bash
    curl http://127.0.0.1:3050/health
    ```

11. Call the proxy with your local proxy token:

    ```bash
    curl http://127.0.0.1:3050/v1/models \
      -H 'Authorization: Bearer <proxy_token>'
    ```

The Docker Compose service listens on `http://127.0.0.1:3050` by default.

## Local Binary Usage

Build the proxy:

```bash
go build -o bin/proxy ./cmd/proxy
```

Run it locally:

```bash
./bin/proxy -config data/config.json
```

## Configuration

The proxy loads `config.json` by default and then applies environment variable overrides. The Docker image runs with `-config /app/config.json`, and `docker-compose.yml` mounts `./data/config.json` to `/app/config.json`.

Example configuration:

```json
{
  "port": 3050,
  "host": "0.0.0.0",
  "cc_apiKey": "user_xxxxxxxxx",
  "apiBase": "https://api.commandcode.ai",
  "projectSlug": "",
  "proxy_token": "test",
  "logFile": "data/logs/proxy.log",
  "logLevel": "info"
}
```

| Field                    | Purpose                                                                                                              |
|--------------------------|----------------------------------------------------------------------------------------------------------------------|
| `port`                   | Local listen port. Overridden by `PORT`.                                                                             |
| `host`                   | Local listen address. Overridden by `HOST`.                                                                          |
| `apiBase`                | Upstream CommandCode API base URL. Overridden by `COMMANDCODE_API_BASE`.                                             |
| `cc_apiKey`              | Optional fallback upstream CommandCode credential. Must contain a `user_` key when used.                             |
| `proxy_token`            | Optional local proxy authentication token for clients. Overridden by `COMMANDCODE_PROXY_TOKEN`.                      |
| `projectSlug`            | Optional explicit upstream project slug. Empty value uses a session-derived fake slug. Overridden by `PROJECT_SLUG`. |
| `logFile`                | Optional log file path. Overridden by `LOG_FILE`.                                                                    |
| `logLevel`               | Log level. Overridden by `LOG_LEVEL`.                                                                                |
| `useProviderModels`      | Enables dynamic model fetching from the Provider API. Overridden by `COMMANDCODE_USE_PROVIDER_MODELS`.               |
| `modelRefreshIntervalMs` | Provider model refresh interval in milliseconds.                                                                     |
| `fingerprint`            | Persisted machine fingerprint generated on first run when absent.                                                    |

## Credential Model

`cc_apiKey` and `proxy_token` are different credentials and must not be mixed.

| Credential    | Used by                          | Purpose                                          |
|---------------|----------------------------------|--------------------------------------------------|
| `proxy_token` | Local clients calling this proxy | Authenticates access to the local proxy.         |
| `cc_apiKey`   | This proxy calling CommandCode   | Authenticates upstream CommandCode API requests. |

When `proxy_token` is configured, clients must send one of:

```http
Authorization: Bearer <proxy_token>
```

or:

```http
X-Proxy-Token: <proxy_token>
```

The proxy then uses `cc_apiKey` from config for upstream CommandCode requests.

When `proxy_token` is not configured, the proxy extracts the first `user_[a-zA-Z0-9_-]+` key from the incoming bearer token and rejects requests that do not contain a `user_` key. `sk-...` keys are not valid CommandCode credentials.

## Environment Variables

| Variable                          | Overrides           |
|-----------------------------------|---------------------|
| `PORT`                            | `port`              |
| `HOST`                            | `host`              |
| `COMMANDCODE_API_BASE`            | `apiBase`           |
| `COMMANDCODE_PROXY_TOKEN`         | `proxy_token`       |
| `PROJECT_SLUG`                    | `projectSlug`       |
| `LOG_FILE`                        | `logFile`           |
| `LOG_LEVEL`                       | `logLevel`          |
| `COMMANDCODE_USE_PROVIDER_MODELS` | `useProviderModels` |

Use the `COMMANDCODE_` prefix for CommandCode-specific environment variables.

## API Endpoints

| Endpoint                    | Auth | Description                                  |
|-----------------------------|------|----------------------------------------------|
| `GET /health`               | No   | Health check.                                |
| `GET /v1/models`            | Yes  | Returns OpenAI-compatible model list data.   |
| `POST /v1/chat/completions` | Yes  | OpenAI Chat Completions compatible endpoint. |
| `POST /v1/messages`         | Yes  | Anthropic Messages compatible endpoint.      |

Protected routes reject invalid authentication before forwarding upstream.

## OpenAI-Compatible Usage

Non-streaming example:

```bash
curl http://127.0.0.1:3050/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer test' \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "messages": [
      {"role": "user", "content": "selamun aleykum"}
    ]
  }'
```

Streaming example:

```bash
curl http://127.0.0.1:3050/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer test' \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "stream": true,
    "messages": [
      {"role": "user", "content": "Write a short greeting."}
    ]
  }'
```

## Anthropic-Compatible Usage

Non-streaming example:

```bash
curl http://127.0.0.1:3050/v1/messages \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer test' \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "max_tokens": 256,
    "messages": [
      {"role": "user", "content": "Write a short greeting."}
    ]
  }'
```

Streaming example:

```bash
curl http://127.0.0.1:3050/v1/messages \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer test' \
  -d '{
    "model": "deepseek/deepseek-v4-flash",
    "max_tokens": 256,
    "stream": true,
    "messages": [
      {"role": "user", "content": "Write a short greeting."}
    ]
  }'
```

## Tool Calling

Both compatible endpoints support tool calling in streaming and non-streaming responses.

OpenAI requests use `tools` with `type: "function"` and OpenAI-style `tool_choice`. Anthropic requests use `tools` with `input_schema` and Anthropic-style `tool_choice`.

When serving Anthropic `/v1/messages`, OpenAI response `tool_calls` are converted to Anthropic `tool_use` content blocks.

## Image Inputs

Protocol conversion supports:

- OpenAI `image_url` content blocks.
- Anthropic URL image sources.
- Anthropic base64 image sources through `source.data` and `source.media_type`, converted into OpenAI data URLs.

Model support for image inputs depends on the selected upstream model.

## Docker Deployment

`docker-compose.yml` defines:

| Setting              | Value                                 |
|----------------------|---------------------------------------|
| Compose project name | `commandcode-bridge`                  |
| Service              | `proxy`                               |
| Container name       | `commandcode-bridge-proxy`            |
| Host port            | `3050`                                |
| Container port       | `3000`                                |
| Runtime config mount | `./data/config.json:/app/config.json` |
| Runtime logs mount   | `./data/logs:/app/data/logs`          |

Start the service:

```bash
docker compose up -d
```

Rebuild after code changes:

```bash
docker compose up -d --build
```

Check health:

```bash
curl http://127.0.0.1:3050/health
```

## Build and Development Commands

| Command                                       | Purpose                                          |
|-----------------------------------------------|--------------------------------------------------|
| `go build -o bin/proxy ./cmd/proxy`           | Build the local binary.                          |
| `go run ./cmd/proxy`                          | Run from source using default config resolution. |
| `go run ./cmd/proxy -config data/config.json` | Run from source with explicit config path.       |
| `go test ./...`                               | Run the full Go test suite.                      |
| `go test ./internal/protocol -run TestName`   | Run one protocol test.                           |
| `go test ./internal/http -run TestName`       | Run one HTTP handler or middleware test.         |
| `go test ./internal/streaming -run TestName`  | Run one streaming translator test.               |
| `go vet ./...`                                | Run Go static checks.                            |
| `gofmt -w <files>`                            | Format changed Go files.                         |

There is no Makefile or package manager manifest in this repository.

## Architecture

High-level request flow:

1. `cmd/proxy/main.go` loads config, initializes logging, loads or creates the fingerprint, creates the HTTP client, session store, init manager, and model manager, refreshes the Command Code CLI version, then starts the server.
2. `internal/http/server.go` builds the chi router and attaches middleware for CORS, request size limit, request timeout, logging, and authentication.
3. `internal/http/handlers.go` handles endpoint orchestration: decode request body, initialize upstream fingerprint and lifecycle state, resolve session ID, convert payload, forward upstream, and translate the response.
4. `internal/protocol` owns OpenAI, Anthropic, and CommandCode request and response conversion.
5. `internal/streaming` converts upstream NDJSON stream events into OpenAI SSE or Anthropic SSE events and applies stream idle timeouts.
6. `internal/client` is the upstream HTTP boundary. It forwards chat requests to `/alpha/generate`, fetches Provider API models, sends fingerprint and lifecycle events, and controls upstream headers.
7. `internal/models` caches Provider API model data and refreshes it when dynamic model fetching is enabled.
8. `internal/session` maps API keys and incoming session headers to stable session IDs with expiry and jitter.
9. `internal/fingerprint` and `internal/config` build runtime environment data and persisted fingerprint values.
10. `pkg/version` manages the Command Code CLI version and refreshes it from the npm registry.

## Upstream Compatibility Notes

- Chat requests are forwarded to upstream `/alpha/generate`.
- Upstream chat responses are treated as NDJSON streams, even for non-streaming client requests.
- Upstream `permissionMode` is `standard`.
- Omitted or non-positive OpenAI `max_tokens` defaults to `64000`.
- OpenAI `max_tokens` values above `200000` are capped to `200000`.
- Empty `projectSlug` uses a session-derived fake slug; configured `projectSlug` is an explicit override.
- The Command Code CLI version is refreshed from npm before serving requests.
- Client disconnects cancel upstream requests.
- Streaming idle timeout is 30 seconds.
- Non-streaming idle timeout is 90 seconds.
- Zero output tokens return a retryable `429` response.

## Logging and Security

Runtime logs can be written under `data/logs/` through `logFile`.

Do not log or expose:

- API keys or bearer token fragments.
- `cc_apiKey` or `proxy_token` values.
- Raw upstream error bodies.
- Stack traces.
- Request bodies containing user prompts, tool payloads, image URLs, or other user data.

The service accepts user-controlled payloads and forwards them upstream. Preserve request size limits, timeouts, upstream error handling, and header filtering when changing HTTP, protocol, or streaming code.

When adding SQL, shell execution, template rendering, file access, or new outbound HTTP behavior, evaluate the related OWASP Top 10 risks before implementation.
