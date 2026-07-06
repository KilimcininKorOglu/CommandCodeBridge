# CommandCode Bridge

[English](README.md)

CommandCode Bridge, CommandCode Go aboneliğiniz için OpenAI uyumlu ve Anthropic uyumlu HTTP endpointleri sunan bir Go reverse proxy uygulamasıdır.

Yerel client isteklerini kabul eder, OpenAI veya Anthropic payloadlarını CommandCode upstream request biçimine dönüştürür, CommandCode tarafına iletir ve upstream NDJSON yanıtlarını OpenAI veya Anthropic response biçimlerine çevirir.

## Özellikler

- OpenAI uyumlu `POST /v1/chat/completions` endpointi.
- Anthropic uyumlu `POST /v1/messages` endpointi.
- Provider API model listesine dayalı OpenAI uyumlu `GET /v1/models` endpointi.
- Streaming ve non-streaming response desteği.
- OpenAI ve Anthropic response biçimleri için tool calling desteği.
- Anthropic URL ve base64 image source değerlerini OpenAI `image_url` blocklarına dönüştürme desteği.
- Request header ile session reuse destekleyen per-key session management.
- Upstream requestler için machine fingerprint ve CLI compatibility headerları.
- `proxy_token` ile isteğe bağlı local proxy authentication.
- `cc_apiKey` ile isteğe bağlı fallback upstream credential.

## Gereksinimler

- Go `1.26.4` veya daha yeni bir sürüm.
- Container deployment için Docker ve Docker Compose.
- Upstream API erişimi için `user_...` biçiminde bir CommandCode API key.

## Kurulum

1. Node.js ve npm kurulu değilse kurun.

2. Command Code CLI yazılımını kurun:

   ```bash
   npm i -g command-code@latest
   ```

3. Command Code CLI ile giriş yapın:

   ```bash
   cmd login
   ```

4. Örnek config dosyasını kopyalayın:

   ```bash
   cp data/config.example.json data/config.json
   ```

5. `~/.commandcode/auth.json` dosyasındaki `apiKey` değerini `data/config.json` içindeki `cc_apiKey` alanına yazın. `data/config.json` dosyasını gizli tutun.

6. `data/config.json` içindeki `proxy_token` değerini tahmin edilmesi zor bir local proxy token ile değiştirin. Clientlar proxy çağırırken bu tokenı kullanmalıdır.

7. Sabit bir upstream project slug istiyorsanız `projectSlug` değerini ayarlayın. Session-derived fake slug kullanmak için boş bırakın.

8. Logları `data/logs/` altında tutun:

   ```json
   {
     "logFile": "data/logs/proxy.log"
   }
   ```

9. Docker Compose ile build edip başlatın:

   ```bash
   docker compose up -d --build
   ```

10. Servisi doğrulayın:

    ```bash
    curl http://127.0.0.1:3050/health
    ```

11. Proxy çağrılarında local proxy token değerini gönderin:

    ```bash
    curl http://127.0.0.1:3050/v1/models \
      -H 'Authorization: Bearer <proxy_token>'
    ```

Docker Compose servisi varsayılan olarak `http://127.0.0.1:3050` adresinde dinler.

## Local Binary Kullanımı

Proxy binary dosyasını build edin:

```bash
go build -o bin/proxy ./cmd/proxy
```

Local olarak çalıştırın:

```bash
./bin/proxy -config data/config.json
```

## Yapılandırma

Proxy varsayılan olarak `config.json` dosyasını yükler ve ardından environment variable override değerlerini uygular. Docker image `-config /app/config.json` ile çalışır ve `docker-compose.yml`, `./data/config.json` dosyasını `/app/config.json` yoluna mount eder.

Örnek yapılandırma:

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

| Alan                     | Amaç                                                                                                                           |
|--------------------------|--------------------------------------------------------------------------------------------------------------------------------|
| `port`                   | Local listen port. `PORT` ile override edilir.                                                                                 |
| `host`                   | Local listen address. `HOST` ile override edilir.                                                                              |
| `apiBase`                | Upstream CommandCode API base URL. `COMMANDCODE_API_BASE` ile override edilir.                                                 |
| `cc_apiKey`              | İsteğe bağlı fallback upstream CommandCode credential. Kullanıldığında `user_` key içermelidir.                                |
| `proxy_token`            | Clientlar için isteğe bağlı local proxy authentication token. `COMMANDCODE_PROXY_TOKEN` ile override edilir.                   |
| `projectSlug`            | İsteğe bağlı explicit upstream project slug. Boş değer session-derived fake slug kullanır. `PROJECT_SLUG` ile override edilir. |
| `logFile`                | İsteğe bağlı log file path. `LOG_FILE` ile override edilir.                                                                    |
| `logLevel`               | Log level. `LOG_LEVEL` ile override edilir.                                                                                    |
| `useProviderModels`      | Provider API üzerinden dynamic model fetching özelliğini etkinleştirir. `COMMANDCODE_USE_PROVIDER_MODELS` ile override edilir. |
| `modelRefreshIntervalMs` | Provider model refresh interval değeridir, milliseconds cinsindendir.                                                          |
| `fingerprint`            | Yoksa ilk çalıştırmada üretilen persisted machine fingerprint değeridir.                                                       |

## Credential Modeli

`cc_apiKey` ve `proxy_token` farklı credential değerleridir ve karıştırılmamalıdır.

| Credential    | Kullanan taraf                           | Amaç                                                      |
|---------------|------------------------------------------|-----------------------------------------------------------|
| `proxy_token` | Bu proxy’ye istek atan local clientlar   | Local proxy erişimini authenticate eder.                  |
| `cc_apiKey`   | CommandCode tarafına istek atan bu proxy | Upstream CommandCode API requestlerini authenticate eder. |

`proxy_token` yapılandırılmışsa clientlar aşağıdakilerden birini göndermelidir:

```http
Authorization: Bearer <proxy_token>
```

veya:

```http
X-Proxy-Token: <proxy_token>
```

Proxy ardından upstream CommandCode requestleri için config dosyasındaki `cc_apiKey` değerini kullanır.

`proxy_token` yapılandırılmamışsa proxy incoming bearer token içinden ilk `user_[a-zA-Z0-9_-]+` key değerini çıkarır ve `user_` key içermeyen requestleri reddeder. `sk-...` keyleri geçerli CommandCode credential değildir.

## Environment Variables

| Değişken                          | Override ettiği alan |
|-----------------------------------|----------------------|
| `PORT`                            | `port`               |
| `HOST`                            | `host`               |
| `COMMANDCODE_API_BASE`            | `apiBase`            |
| `COMMANDCODE_PROXY_TOKEN`         | `proxy_token`        |
| `PROJECT_SLUG`                    | `projectSlug`        |
| `LOG_FILE`                        | `logFile`            |
| `LOG_LEVEL`                       | `logLevel`           |
| `COMMANDCODE_USE_PROVIDER_MODELS` | `useProviderModels`  |

CommandCode özel environment variable değerleri için `COMMANDCODE_` prefixini kullanın.

## API Endpointleri

| Endpoint                    | Auth  | Açıklama                                 |
|-----------------------------|-------|------------------------------------------|
| `GET /health`               | Hayır | Health check.                            |
| `GET /v1/models`            | Evet  | OpenAI uyumlu model list data döndürür.  |
| `POST /v1/chat/completions` | Evet  | OpenAI Chat Completions uyumlu endpoint. |
| `POST /v1/messages`         | Evet  | Anthropic Messages uyumlu endpoint.      |

Protected route değerleri upstream tarafına forward edilmeden önce invalid authentication formatlarını reddeder.

## OpenAI Uyumlu Kullanım

Non-streaming örneği:

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

Streaming örneği:

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

## Anthropic Uyumlu Kullanım

Non-streaming örneği:

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

Streaming örneği:

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

İki uyumlu endpoint de streaming ve non-streaming response değerlerinde tool calling destekler.

OpenAI requestleri `type: "function"` içeren `tools` ve OpenAI style `tool_choice` kullanır. Anthropic requestleri `input_schema` içeren `tools` ve Anthropic style `tool_choice` kullanır.

Anthropic `/v1/messages` sunulurken OpenAI response `tool_calls` değerleri Anthropic `tool_use` content blocklarına dönüştürülür.

## Image Inputs

Protocol conversion şunları destekler:

- OpenAI `image_url` content blockları.
- Anthropic URL image source değerleri.
- `source.data` ve `source.media_type` üzerinden Anthropic base64 image source değerleri, OpenAI data URL değerlerine dönüştürülür.

Image input desteği seçilen upstream modele bağlıdır.

## Docker Deployment

`docker-compose.yml` şunları tanımlar:

| Ayar                 | Değer                                 |
|----------------------|---------------------------------------|
| Compose project name | `commandcode-bridge`                  |
| Service              | `proxy`                               |
| Container name       | `commandcode-bridge-proxy`            |
| Host port            | `3050`                                |
| Container port       | `3000`                                |
| Runtime config mount | `./data/config.json:/app/config.json` |
| Runtime logs mount   | `./data/logs:/app/data/logs`          |

Servisi başlatın:

```bash
docker compose up -d
```

Kod değişikliklerinden sonra yeniden build edin:

```bash
docker compose up -d --build
```

Health check:

```bash
curl http://127.0.0.1:3050/health
```

## Build ve Development Komutları

| Komut                                         | Amaç                                                          |
|-----------------------------------------------|---------------------------------------------------------------|
| `go build -o bin/proxy ./cmd/proxy`           | Local binary build eder.                                      |
| `go run ./cmd/proxy`                          | Varsayılan config resolution ile source üzerinden çalıştırır. |
| `go run ./cmd/proxy -config data/config.json` | Explicit config path ile source üzerinden çalıştırır.         |
| `go test ./...`                               | Full Go test suite çalıştırır.                                |
| `go test ./internal/protocol -run TestName`   | Tek bir protocol testini çalıştırır.                          |
| `go test ./internal/http -run TestName`       | Tek bir HTTP handler veya middleware testini çalıştırır.      |
| `go test ./internal/streaming -run TestName`  | Tek bir streaming translator testini çalıştırır.              |
| `go vet ./...`                                | Go static checks çalıştırır.                                  |
| `gofmt -w <files>`                            | Değiştirilen Go dosyalarını formatlar.                        |

Bu repository’de Makefile veya package manager manifest yoktur.

## Mimari

High-level request flow:

1. `cmd/proxy/main.go` config yükler, logging başlatır, fingerprint yükler veya oluşturur, HTTP client, session store, init manager ve model manager oluşturur, Command Code CLI version değerini yeniler ve ardından server başlatır.
2. `internal/http/server.go` chi router oluşturur ve CORS, request size limit, request timeout, logging ve authentication middleware ekler.
3. `internal/http/handlers.go` endpoint orchestration işlemini yürütür: request body decode eder, upstream fingerprint ve lifecycle state başlatır, session ID resolve eder, payload dönüştürür, upstream tarafına forward eder ve response çevirir.
4. `internal/protocol` OpenAI, Anthropic ve CommandCode request/response conversion işlemlerinden sorumludur.
5. `internal/streaming` upstream NDJSON stream eventlerini OpenAI SSE veya Anthropic SSE eventlerine dönüştürür ve stream idle timeout uygular.
6. `internal/client` upstream HTTP boundary katmanıdır. Chat requestlerini `/alpha/generate` yoluna forward eder, Provider API modellerini fetch eder, fingerprint ve lifecycle eventleri gönderir ve upstream headerları yönetir.
7. `internal/models` Provider API model data değerlerini cache eder ve dynamic model fetching etkinse yeniler.
8. `internal/session` API keyleri ve incoming session headerlarını expiry ve jitter içeren stable session ID değerlerine map eder.
9. `internal/fingerprint` ve `internal/config` runtime environment data ve persisted fingerprint değerlerini oluşturur.
10. `pkg/version` Command Code CLI version değerini yönetir ve npm registry üzerinden yeniler.

## Upstream Compatibility Notları

- Chat requestleri upstream `/alpha/generate` yoluna forward edilir.
- Upstream chat response değerleri non-streaming client requestleri için bile NDJSON stream olarak işlenir.
- Upstream `permissionMode` değeri `standard` olur.
- OpenAI `max_tokens` eksikse veya pozitif değilse varsayılan değer `64000` olur.
- OpenAI `max_tokens` değeri `200000` üstündeyse `200000` ile sınırlandırılır.
- Boş `projectSlug`, session-derived fake slug kullanır; configured `projectSlug` explicit override olarak davranır.
- Command Code CLI version, request servis edilmeden önce npm üzerinden yenilenir.
- Client disconnect durumunda upstream requestler iptal edilir.
- Streaming idle timeout 30 saniyedir.
- Non-streaming idle timeout 90 saniyedir.
- Zero output token durumunda retryable `429` response döner.

## Logging ve Güvenlik

Runtime logları `logFile` üzerinden `data/logs/` altına yazılabilir.

Şunları loglamayın veya açığa çıkarmayın:

- API keyler veya bearer token parçaları.
- `cc_apiKey` veya `proxy_token` değerleri.
- Raw upstream error body değerleri.
- Stack trace değerleri.
- User prompt, tool payload, image URL veya başka user data içeren request body değerleri.

Servis user-controlled payload kabul eder ve upstream tarafına forward eder. HTTP, protocol veya streaming kodu değiştirirken request size limit, timeout, upstream error handling ve header filtering davranışlarını koruyun.

SQL, shell execution, template rendering, file access veya yeni outbound HTTP behavior eklerken ilgili OWASP Top 10 risklerini implementation öncesinde değerlendirin.

## Lisans

Yalnızca Araştırma
