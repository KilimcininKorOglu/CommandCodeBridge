package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kilimcininkoroglu/commandcode-bridge/internal/client"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/config"
	apierror "github.com/kilimcininkoroglu/commandcode-bridge/internal/error"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/logging"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/models"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/protocol"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/session"
	"github.com/kilimcininkoroglu/commandcode-bridge/internal/streaming"
)

const (
	streamIdleTimeout              = 30 * time.Second
	nonStreamIdleTimeout           = 90 * time.Second
	timeoutReduceContextThreshold  = 3
	emptyResponseRetryAfterSeconds = 10
	timeoutRetryAfterSeconds       = 5
)

var consecutiveTimeouts atomic.Int64

// HandlerDependencies contains dependencies for HTTP handlers
type HandlerDependencies struct {
	Config       *config.Config
	Logger       *logging.Logger
	SessionStore *session.SessionStore
	Client       *client.Client
	ModelManager *models.ModelManager
	InitManager  *client.InitManager
	Version      string
}

// HandleChatCompletions handles OpenAI chat completions endpoint
func HandleChatCompletions(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey := r.Context().Value("ccAPIKey").(string)

		// Ensure initialization (fingerprint/lifecycle events)
		if err := deps.InitManager.EnsureInitialized(r.Context(), ccAPIKey, deps.Config.Fingerprint); err != nil {
			deps.Logger.Warn("Initialization failed", map[string]any{
				"error": err.Error(),
			})
		}

		// Parse request body
		var openaiReq protocol.OpenAIRequest
		if apiErr := decodeRequestBody(r.Body, &openaiReq); apiErr != nil {
			deps.Logger.Warn("Failed to parse OpenAI request body", map[string]any{
				"error": apiErr.Message,
			})
			sendError(w, apiErr)
			return
		}

		deps.Logger.Debug("OpenAI request received", map[string]any{
			"model":               openaiReq.Model,
			"stream":              openaiReq.Stream,
			"max_tokens":          openaiReq.MaxTokens,
			"messages":            len(openaiReq.Messages),
			"tools":               len(openaiReq.Tools),
			"reasoning_effort":    openaiReq.ReasoningEffort,
			"thinking_enabled":    openaiReq.Thinking != nil,
			"parallel_tool_calls": openaiReq.ParallelToolCalls,
		})

		// Get session ID
		sessionID := deps.SessionStore.Resolve(r.Header, ccAPIKey)

		// Convert to CommandCode format
		ccReq, err := protocol.OpenAIToCommandCode(&openaiReq)
		if err != nil {
			deps.Logger.Error("Failed to convert OpenAI request to CommandCode", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to convert request").WithCode(http.StatusInternalServerError))
			return
		}

		// Marshal CommandCode request
		ccBody, err := json.Marshal(ccReq)
		if err != nil {
			deps.Logger.Error("Failed to marshal CommandCode request", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to marshal request").WithCode(http.StatusInternalServerError))
			return
		}

		// Forward to CommandCode API
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		resp, err := deps.Client.Forward(ctx, ccBody, ccAPIKey, r.Header, sessionID, deps.Config.Fingerprint)
		if err != nil {
			deps.Logger.Error("Failed to forward request", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to forward request").WithCode(http.StatusBadGateway))
			return
		}
		defer resp.Body.Close()

		// Handle streaming response
		if openaiReq.Stream {
			if handled := handleUpstreamStatus(w, resp, deps.Logger); handled {
				return
			}

			translator := streaming.NewOpenAITranslator(openaiReq.Model, generateCompletionID(), time.Now().Unix())
			streamWriter := newDelayedSSEWriter(w)
			if err := translator.TranslateStreamWithIdleTimeout(resp.Body, streamWriter, streamIdleTimeout); err != nil {
				handleStreamingError(w, streamWriter, deps.Logger, err)
				return
			}
			consecutiveTimeouts.Store(0)
			inputTokens, outputTokens, cachedTokens := translator.GetUsage()
			if outputTokens == 0 {
				cancel()
				deps.Logger.Warn("Zero output tokens from upstream (OpenAI streaming)", nil)
				sendStreamZeroOutput(w, streamWriter)
				return
			}
			deps.Logger.Info("Token usage", map[string]any{
				"endpoint":      "openai/chat/completions",
				"stream":        true,
				"model":         openaiReq.Model,
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
				"cached_tokens": cachedTokens,
			})
			streamWriter.WriteDone()
			return
		}

		// Handle non-streaming response
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				deps.Logger.Error("Failed to read upstream response body", map[string]any{
					"error": err.Error(),
				})
				sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to read response").WithCode(http.StatusBadGateway))
				return
			}
			apiErr := apierror.MapStatus(resp.StatusCode, string(body))
			deps.Logger.Warn("Upstream non-200 response", map[string]any{
				"status": resp.StatusCode,
			})
			sendError(w, apiErr)
			return
		}

		openaiResp, err := commandCodeStreamToOpenAIWithIdleTimeout(resp.Body, openaiReq.Model, generateCompletionID(), time.Now().Unix(), deps.Logger, nonStreamIdleTimeout)
		if err != nil {
			if apiErr, ok := err.(*apierror.APIError); ok {
				sendError(w, apiErr)
				return
			}
			deps.Logger.Error("Failed to parse upstream response", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to parse response").WithCode(http.StatusBadGateway))
			return
		}

		if openaiResp.Usage != nil {
			deps.Logger.Info("Token usage", map[string]any{
				"endpoint":      "openai/chat/completions",
				"stream":        false,
				"model":         openaiReq.Model,
				"input_tokens":  openaiResp.Usage.InputTokens,
				"output_tokens": openaiResp.Usage.OutputTokens,
				"cached_tokens": openaiResp.Usage.CachedInputTokens,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openaiResp)
	}
}

// HandleResponses handles OpenAI Responses endpoint.
func HandleResponses(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey := r.Context().Value("ccAPIKey").(string)
		if err := deps.InitManager.EnsureInitialized(r.Context(), ccAPIKey, deps.Config.Fingerprint); err != nil {
			deps.Logger.Warn("Initialization failed", map[string]any{"error": err.Error()})
		}

		var req protocol.OpenAIResponsesRequest
		if apiErr := decodeRequestBody(r.Body, &req); apiErr != nil {
			deps.Logger.Warn("Failed to parse OpenAI Responses request body", map[string]any{"error": apiErr.Message})
			sendError(w, apiErr)
			return
		}

		deps.Logger.Debug("OpenAI Responses request received", map[string]any{
			"model":               req.Model,
			"stream":              req.Stream,
			"max_output_tokens":   req.MaxOutputTokens,
			"tools":               len(req.Tools),
			"reasoning_effort":    req.ReasoningEffort,
			"thinking_enabled":    req.Thinking != nil,
			"parallel_tool_calls": req.ParallelToolCalls,
		})

		ccReq, err := protocol.OpenAIResponsesToCommandCode(&req)
		if err != nil {
			deps.Logger.Error("Failed to convert OpenAI Responses request to CommandCode", map[string]any{"error": err.Error()})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to convert request").WithCode(http.StatusInternalServerError))
			return
		}
		handleCommandCodeResponsesRequest(w, r, deps, ccAPIKey, req.Model, req.Stream, ccReq, false)
	}
}

// HandleResponsesCompact handles OpenAI Responses compaction endpoint.
func HandleResponsesCompact(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey := r.Context().Value("ccAPIKey").(string)
		if err := deps.InitManager.EnsureInitialized(r.Context(), ccAPIKey, deps.Config.Fingerprint); err != nil {
			deps.Logger.Warn("Initialization failed", map[string]any{"error": err.Error()})
		}

		var req protocol.OpenAIResponsesCompactRequest
		if apiErr := decodeRequestBody(r.Body, &req); apiErr != nil {
			deps.Logger.Warn("Failed to parse OpenAI Responses compact request body", map[string]any{"error": apiErr.Message})
			sendError(w, apiErr)
			return
		}

		deps.Logger.Debug("OpenAI Responses compact request received", map[string]any{
			"model": req.Model,
		})

		ccReq, err := protocol.OpenAIResponsesCompactToCommandCode(&req)
		if err != nil {
			deps.Logger.Error("Failed to convert OpenAI Responses compact request to CommandCode", map[string]any{"error": err.Error()})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to convert request").WithCode(http.StatusInternalServerError))
			return
		}
		handleCommandCodeResponsesRequest(w, r, deps, ccAPIKey, req.Model, false, ccReq, true)
	}
}

func handleCommandCodeResponsesRequest(w http.ResponseWriter, r *http.Request, deps *HandlerDependencies, ccAPIKey string, model string, stream bool, ccReq *protocol.CommandCodeRequest, compact bool) {
	sessionID := deps.SessionStore.Resolve(r.Header, ccAPIKey)
	ccBody, err := json.Marshal(ccReq)
	if err != nil {
		deps.Logger.Error("Failed to marshal CommandCode request", map[string]any{"error": err.Error()})
		sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to marshal request").WithCode(http.StatusInternalServerError))
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	resp, err := deps.Client.Forward(ctx, ccBody, ccAPIKey, r.Header, sessionID, deps.Config.Fingerprint)
	if err != nil {
		deps.Logger.Error("Failed to forward request", map[string]any{"error": err.Error()})
		sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to forward request").WithCode(http.StatusBadGateway))
		return
	}
	defer resp.Body.Close()

	if stream {
		if handled := handleUpstreamStatus(w, resp, deps.Logger); handled {
			return
		}
		streamWriter := newDelayedSSEWriter(w)
		responseID := protocol.ResponseID("resp")
		if err := commandCodeStreamToOpenAIResponsesSSE(resp.Body, streamWriter, responseID, model, time.Now().Unix(), deps.Logger, streamIdleTimeout); err != nil {
			handleStreamingError(w, streamWriter, deps.Logger, err)
			return
		}
		consecutiveTimeouts.Store(0)
		streamWriter.WriteDone()
		return
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			deps.Logger.Error("Failed to read upstream response body", map[string]any{"error": err.Error()})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to read response").WithCode(http.StatusBadGateway))
			return
		}
		deps.Logger.Warn("Upstream non-200 response", map[string]any{"status": resp.StatusCode})
		sendError(w, apierror.MapStatus(resp.StatusCode, string(body)))
		return
	}

	openaiResp, err := commandCodeStreamToOpenAIWithIdleTimeout(resp.Body, model, generateCompletionID(), time.Now().Unix(), deps.Logger, nonStreamIdleTimeout)
	if err != nil {
		if apiErr, ok := err.(*apierror.APIError); ok {
			sendError(w, apiErr)
			return
		}
		deps.Logger.Error("Failed to parse upstream response", map[string]any{"error": err.Error()})
		sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to parse response").WithCode(http.StatusBadGateway))
		return
	}

	var responseObject *protocol.OpenAIResponseObject
	message := openaiResp.Choices[0].Message
	if compact {
		responseObject = protocol.BuildOpenAICompactionObject(protocol.ResponseID("resp"), time.Now().Unix(), fmt.Sprint(message.Content), openaiResp.Usage)
	} else {
		responseObject = protocol.BuildOpenAIResponseObject(protocol.ResponseID("resp"), model, time.Now().Unix(), fmt.Sprint(message.Content), message.ToolCalls, openaiResp.Usage)
	}
		if openaiResp.Usage != nil {
			deps.Logger.Info("Token usage", map[string]any{
				"endpoint":      "openai/responses",
				"stream":        false,
				"model":         model,
				"input_tokens":  openaiResp.Usage.InputTokens,
				"output_tokens": openaiResp.Usage.OutputTokens,
				"cached_tokens": openaiResp.Usage.CachedInputTokens,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responseObject)
}

// HandleMessages handles Anthropic messages endpoint
func HandleMessages(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey := r.Context().Value("ccAPIKey").(string)

		// Ensure initialization (fingerprint/lifecycle events)
		if err := deps.InitManager.EnsureInitialized(r.Context(), ccAPIKey, deps.Config.Fingerprint); err != nil {
			deps.Logger.Warn("Initialization failed", map[string]any{
				"error": err.Error(),
			})
		}

		// Parse request body
		var anthropicReq protocol.AnthropicRequest
		if apiErr := decodeRequestBody(r.Body, &anthropicReq); apiErr != nil {
			deps.Logger.Warn("Failed to parse Anthropic request body", map[string]any{
				"error": apiErr.Message,
			})
			sendError(w, apiErr)
			return
		}

		thinkingType := ""
		thinkingEffort := ""
		thinkingBudgetTokens := 0
		if anthropicReq.Thinking != nil {
			thinkingType = anthropicReq.Thinking.Type
			thinkingEffort = anthropicReq.Thinking.Effort
			thinkingBudgetTokens = anthropicReq.Thinking.BudgetTokens
		}

		deps.Logger.Debug("Anthropic request received", map[string]any{
			"model":                  anthropicReq.Model,
			"stream":                 anthropicReq.Stream,
			"max_tokens":             anthropicReq.MaxTokens,
			"messages":               len(anthropicReq.Messages),
			"tools":                  len(anthropicReq.Tools),
			"thinking_type":          thinkingType,
			"thinking_effort":        thinkingEffort,
			"thinking_budget_tokens": thinkingBudgetTokens,
		})

		// Get session ID
		sessionID := deps.SessionStore.Resolve(r.Header, ccAPIKey)

		// Convert to CommandCode format
		ccReq, err := protocol.AnthropicMessagesToCommandCode(&anthropicReq)
		if err != nil {
			deps.Logger.Error("Failed to convert Anthropic request to CommandCode", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to convert request").WithCode(http.StatusInternalServerError))
			return
		}

		deps.Logger.Debug("CommandCode request prepared", map[string]any{
			"reasoning_effort": ccReq.Params.ReasoningEffort,
			"thinking_enabled": ccReq.Params.Thinking != nil,
		})

		// Marshal CommandCode request
		ccBody, err := json.Marshal(ccReq)
		if err != nil {
			deps.Logger.Error("Failed to marshal CommandCode request", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to marshal request").WithCode(http.StatusInternalServerError))
			return
		}

		// Forward to CommandCode API
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		resp, err := deps.Client.Forward(ctx, ccBody, ccAPIKey, r.Header, sessionID, deps.Config.Fingerprint)
		if err != nil {
			deps.Logger.Error("Failed to forward request", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to forward request").WithCode(http.StatusBadGateway))
			return
		}
		defer resp.Body.Close()

		// Handle streaming response
		if anthropicReq.Stream {
			if handled := handleUpstreamStatus(w, resp, deps.Logger); handled {
				return
			}

			translator := streaming.NewAnthropicTranslator(anthropicReq.Model, generateMessageID())
			streamWriter := newDelayedSSEWriter(w)
			if err := translator.TranslateWithIdleTimeout(resp.Body, streamWriter, streamIdleTimeout); err != nil {
				handleStreamingError(w, streamWriter, deps.Logger, err)
				return
			}
			consecutiveTimeouts.Store(0)
			inputTokens, outputTokens, cachedTokens := translator.GetUsage()
			if outputTokens == 0 {
				cancel()
				deps.Logger.Warn("Zero output tokens from upstream (Anthropic streaming)", nil)
				sendStreamZeroOutput(w, streamWriter)
				return
			}
			deps.Logger.Info("Token usage", map[string]any{
				"endpoint":      "anthropic/messages",
				"stream":        true,
				"model":         anthropicReq.Model,
				"input_tokens":  inputTokens,
				"output_tokens": outputTokens,
				"cached_tokens": cachedTokens,
			})
			streamWriter.WriteDone()
			return
		}

		// Handle non-streaming response
		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				deps.Logger.Error("Failed to read upstream response body", map[string]any{
					"error": err.Error(),
				})
				sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to read response").WithCode(http.StatusBadGateway))
				return
			}
			apiErr := apierror.MapStatus(resp.StatusCode, string(body))
			deps.Logger.Warn("Upstream non-200 response", map[string]any{
				"status": resp.StatusCode,
			})
			sendError(w, apiErr)
			return
		}

		anthropicResp, err := commandCodeStreamToAnthropicMessagesWithIdleTimeout(resp.Body, anthropicReq.Model, generateMessageID(), deps.Logger, nonStreamIdleTimeout)
		if err != nil {
			if apiErr, ok := err.(*apierror.APIError); ok {
				sendError(w, apiErr)
				return
			}
			deps.Logger.Error("Failed to parse upstream response", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to parse response").WithCode(http.StatusBadGateway))
			return
		}

		if anthropicResp.Usage != nil {
			deps.Logger.Info("Token usage", map[string]any{
				"endpoint":      "anthropic/messages",
				"stream":        false,
				"model":         anthropicReq.Model,
				"input_tokens":  anthropicResp.Usage.InputTokens,
				"output_tokens": anthropicResp.Usage.OutputTokens,
				"cached_tokens": anthropicResp.Usage.CacheReadInputTokens,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anthropicResp)
	}
}

// HandleMessagesCountTokens handles Anthropic token counting requests.
func HandleMessagesCountTokens(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey := r.Context().Value("ccAPIKey").(string)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			deps.Logger.Warn("Failed to read Anthropic token count request body", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInvalidRequest, "Invalid request body").WithCode(http.StatusBadRequest))
			return
		}

		var req protocol.AnthropicRequest
		if apiErr := decodeRequestBody(bytes.NewReader(body), &req); apiErr != nil {
			deps.Logger.Warn("Failed to parse Anthropic token count request body", map[string]any{
				"error": apiErr.Message,
			})
			sendError(w, apiErr)
			return
		}

		deps.Logger.Debug("Anthropic token count request received", map[string]any{
			"model":    req.Model,
			"messages": len(req.Messages),
			"tools":    len(req.Tools),
		})

		sessionID := deps.SessionStore.Resolve(r.Header, ccAPIKey)
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		resp, err := deps.Client.ForwardAnthropicCountTokens(ctx, body, ccAPIKey, r.Header, sessionID, deps.Config.Fingerprint, r.URL.RawQuery)
		if err != nil {
			deps.Logger.Error("Failed to forward token count request", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to forward request").WithCode(http.StatusBadGateway))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				deps.Logger.Error("Failed to read upstream token count response body", map[string]any{
					"error": err.Error(),
				})
				sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to read response").WithCode(http.StatusBadGateway))
				return
			}
			apiErr := apierror.MapStatus(resp.StatusCode, string(body))
			deps.Logger.Warn("Upstream token count non-200 response", map[string]any{
				"status": resp.StatusCode,
			})
			sendError(w, apiErr)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		written, err := io.Copy(w, resp.Body)
		if err != nil {
			deps.Logger.Error("Failed to write token count response", map[string]any{
				"error": err.Error(),
			})
			return
		}
		deps.Logger.Debug("Anthropic token count response completed", map[string]any{
			"status": resp.StatusCode,
			"bytes":  written,
		})
	}
}

// HandleModels handles the models endpoint
func HandleModels(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ccAPIKey, _ := extractCCAPIKey(deps.Config.CCAPIKey)

		// Get models
		modelData, err := deps.ModelManager.GetModels(r.Context(), ccAPIKey)
		if err != nil {
			deps.Logger.Error("Failed to get models", map[string]any{
				"error": err.Error(),
			})
			sendError(w, apierror.NewAPIError(apierror.ErrorTypeInternal, "Failed to get models").WithCode(http.StatusInternalServerError))
			return
		}

		resp := map[string]any{
			"object": "list",
			"data":   modelData,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// HandleHealth handles the health check endpoint
func HandleHealth(deps *HandlerDependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": deps.Version,
		})
	}
}

// commandCodeStreamToOpenAI buffers CommandCode NDJSON into a non-streaming OpenAI response.
type delayedSSEWriter struct {
	writer  http.ResponseWriter
	started bool
}

func newDelayedSSEWriter(w http.ResponseWriter) *delayedSSEWriter {
	return &delayedSSEWriter{writer: w}
}

func (w *delayedSSEWriter) Write(p []byte) (int, error) {
	if !w.started {
		w.writer.Header().Set("Content-Type", "text/event-stream")
		w.writer.Header().Set("Cache-Control", "no-cache")
		w.writer.Header().Set("Connection", "keep-alive")
		w.writer.Header().Set("X-Accel-Buffering", "no")
		w.writer.WriteHeader(http.StatusOK)
		w.started = true
	}
	n, err := w.writer.Write(p)
	if err != nil {
		return n, err
	}
	if flusher, ok := w.writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, nil
}

func (w *delayedSSEWriter) Started() bool {
	return w.started
}

func (w *delayedSSEWriter) WriteDone() {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

func handleUpstreamStatus(w http.ResponseWriter, resp *http.Response, logger *logging.Logger) bool {
	if resp.StatusCode == http.StatusOK {
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		sendError(w, apierror.NewAPIError(apierror.ErrorTypeUpstream, "Failed to read response").WithCode(http.StatusBadGateway))
		return true
	}
	logger.Warn("Upstream non-200 response", map[string]any{
		"status": resp.StatusCode,
	})
	sendError(w, apierror.MapStatus(resp.StatusCode, string(body)))
	return true
}

func sendStreamZeroOutput(w http.ResponseWriter, streamWriter *delayedSSEWriter) {
	apiErr := apierror.NewAPIError(apierror.ErrorTypeRateLimit, "Empty response from upstream (zero output tokens)").WithCode(http.StatusTooManyRequests)
	if !streamWriter.Started() {
		sendErrorWithRetryAfter(w, apiErr, emptyResponseRetryAfterSeconds)
		return
	}
	data, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": apiErr.Message,
			"type":    apiErr.Type,
		},
		"retry_after": emptyResponseRetryAfterSeconds,
	})
	_, _ = fmt.Fprintf(streamWriter.writer, "data: %s\n\n", string(data))
}

func timeoutMessage(count int64) string {
	if count >= timeoutReduceContextThreshold {
		return "Response timeout - try reducing context length (summarize earlier messages)"
	}
	return "Response timeout - request timed out"
}

func handleStreamingError(w http.ResponseWriter, streamWriter *delayedSSEWriter, logger *logging.Logger, err error) {
	logger.Error("Streaming error", map[string]any{
		"error": err.Error(),
	})
	if errors.Is(err, streaming.ErrStreamIdleTimeout) {
		count := consecutiveTimeouts.Add(1)
		logger.Warn("Stream idle timeout", map[string]any{
			"consecutive": count,
		})
		sendStreamRetryableError(w, streamWriter, timeoutMessage(count), timeoutRetryAfterSeconds, http.StatusTooManyRequests)
		return
	}
	if !streamWriter.Started() {
		sendErrorWithRetryAfter(w, apierror.NewAPIError(apierror.ErrorTypeProxy, "Upstream streaming error").WithCode(http.StatusBadGateway), emptyResponseRetryAfterSeconds)
	}
}

func sendStreamRetryableError(w http.ResponseWriter, streamWriter *delayedSSEWriter, message string, retryAfter int, status int) {
	apiErr := apierror.NewAPIError(apierror.ErrorTypeRateLimit, message).WithCode(status)
	if !streamWriter.Started() {
		sendErrorWithRetryAfter(w, apiErr, retryAfter)
		return
	}
	data, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": apiErr.Message,
			"type":    apiErr.Type,
		},
		"retry_after": retryAfter,
	})
	_, _ = fmt.Fprintf(streamWriter.writer, "data: %s\n\n", string(data))
}

func decodeRequestBody(body io.Reader, target any) *apierror.APIError {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			return apierror.NewAPIError(apierror.ErrorTypeInvalidRequest, "Request body exceeds 10MB limit").WithCode(http.StatusRequestEntityTooLarge)
		}
		return apierror.NewAPIError(apierror.ErrorTypeInvalidRequest, "Invalid request body").WithCode(http.StatusBadRequest)
	}
	return nil
}

func commandCodeStreamToOpenAI(reader io.Reader, model string, completionID string, created int64, logger *logging.Logger) (*protocol.OpenAIResponse, error) {
	return commandCodeStreamToOpenAIWithIdleTimeout(reader, model, completionID, created, logger, 0)
}

func commandCodeStreamToOpenAIResponsesSSE(reader io.Reader, writer io.Writer, responseID string, model string, created int64, logger *logging.Logger, idleTimeout time.Duration) error {
	createdEvent := protocol.BuildOpenAIResponseObject(responseID, model, created, "", nil, nil)
	if err := writeResponseSSEEvent(writer, "response.created", createdEvent); err != nil {
		return err
	}

	lines := streaming.ScanLines(reader)
	var text strings.Builder
	usage := &protocol.Usage{}
	outputIndex := 0
	contentIndex := 0

	for {
		line, err := streaming.NextLine(lines, idleTimeout)
		if err != nil {
			return err
		}
		if line == "" {
			break
		}
		if line == "[DONE]" {
			continue
		}

		var event streaming.CommandCodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return err
		}

		switch event.Type {
		case "text-delta":
			delta := streaming.TextDeltaContent(event)
			if delta == "" {
				continue
			}
			text.WriteString(delta)
			if err := writeResponseSSEEvent(writer, "response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       responseID,
				"output_index":  outputIndex,
				"content_index": contentIndex,
				"delta":         delta,
			}); err != nil {
				return err
			}
		case "delta":
			delta := event.Delta
			if delta == "" {
				continue
			}
			text.WriteString(delta)
			if err := writeResponseSSEEvent(writer, "response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       responseID,
				"output_index":  outputIndex,
				"content_index": contentIndex,
				"delta":         delta,
			}); err != nil {
				return err
			}
		case "text":
			delta := event.Text
			if delta == "" {
				continue
			}
			text.WriteString(delta)
			if err := writeResponseSSEEvent(writer, "response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_id":       responseID,
				"output_index":  outputIndex,
				"content_index": contentIndex,
				"delta":         delta,
			}); err != nil {
				return err
			}
		case "tool-call", "toolCall":
			arguments := "{}"
			if event.Input != nil {
				if inputString, ok := event.Input.(string); ok {
					arguments = inputString
				} else if inputBytes, err := json.Marshal(event.Input); err == nil {
					arguments = string(inputBytes)
				}
			}
			if err := writeResponseSSEEvent(writer, "response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": outputIndex,
				"item": map[string]any{
					"id":        event.ToolCallID,
					"type":      "function_call",
					"status":    "completed",
					"call_id":   event.ToolCallID,
					"name":      event.ToolName,
					"arguments": arguments,
				},
			}); err != nil {
				return err
			}
			outputIndex++
		case "finish", "finish-step":
			if event.TotalUsage != nil {
				usage.InputTokens = event.TotalUsage.InputTokens
				usage.OutputTokens = event.TotalUsage.OutputTokens
				usage.CachedInputTokens = event.TotalUsage.CachedInputTokens
			} else if event.Usage != nil {
				usage.InputTokens = event.Usage.InputTokens
				usage.OutputTokens = event.Usage.OutputTokens
				usage.CachedInputTokens = event.Usage.CachedInputTokens
			}
		case "error":
			if logger != nil {
				logger.Warn("CommandCode stream error event", map[string]any{
					"message": commandCodeEventMessage(event),
				})
			}
		}
	}

	if usage.OutputTokens == 0 {
		if logger != nil {
			logger.Warn("Zero output tokens from upstream (streaming)", nil)
		}
		return apierror.NewAPIError(apierror.ErrorTypeRateLimit, "Empty response from upstream (zero output tokens)").WithCode(http.StatusTooManyRequests)
	}

	if err := writeResponseSSEEvent(writer, "response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       responseID,
		"output_index":  0,
		"content_index": 0,
		"text":          text.String(),
	}); err != nil {
		return err
	}
	completed := protocol.BuildOpenAIResponseObject(responseID, model, created, text.String(), nil, usage)
	if logger != nil {
		logger.Info("Token usage", map[string]any{
			"endpoint":      "openai/responses",
			"stream":        true,
			"model":         model,
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"cached_tokens": usage.CachedInputTokens,
		})
	}
	return writeResponseSSEEvent(writer, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": completed,
	})
}

func writeResponseSSEEvent(writer io.Writer, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "event: %s\n", event); err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", string(payload))
	return err
}

func commandCodeStreamToAnthropicMessagesWithIdleTimeout(reader io.Reader, model string, messageID string, logger *logging.Logger, idleTimeout time.Duration) (*protocol.AnthropicResponse, error) {
	lines := streaming.ScanLines(reader)
	content, finishReason, usage, err := collectCommandCodeAnthropicContent(lines, logger, idleTimeout)
	if err != nil {
		return nil, err
	}
	return protocol.CommandCodeToAnthropicMessages(messageID, model, content, finishReason, usage), nil
}

func collectCommandCodeAnthropicContent(lines <-chan streaming.StreamLine, logger *logging.Logger, idleTimeout time.Duration) ([]protocol.AnthropicContent, string, *protocol.Usage, error) {
	var textContent strings.Builder
	var reasoningContent strings.Builder
	finishReason := "stop"
	usage := &protocol.Usage{}
	content := []protocol.AnthropicContent{}

	flushText := func() {
		if text := textContent.String(); text != "" {
			content = append(content, protocol.AnthropicContent{Type: "text", Text: text})
			textContent.Reset()
		}
	}
	flushReasoning := func() {
		if thinking := reasoningContent.String(); thinking != "" {
			content = append(content, protocol.AnthropicContent{Type: "thinking", Thinking: thinking})
			reasoningContent.Reset()
		}
	}

	for {
		line, err := streaming.NextLine(lines, idleTimeout)
		if err != nil {
			if errors.Is(err, streaming.ErrStreamIdleTimeout) {
				count := consecutiveTimeouts.Add(1)
				message := timeoutMessage(count)
				return nil, "", nil, apierror.NewAPIError(apierror.ErrorTypeRateLimit, message).WithCode(http.StatusTooManyRequests)
			}
			return nil, "", nil, err
		}
		if line == "" {
			break
		}
		if line == "" || line == "[DONE]" {
			continue
		}

		var event streaming.CommandCodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, "", nil, err
		}

		switch event.Type {
		case "text-delta":
			flushReasoning()
			textContent.WriteString(streaming.TextDeltaContent(event))
		case "text":
			flushReasoning()
			textContent.WriteString(event.Text)
		case "delta":
			flushReasoning()
			textContent.WriteString(event.Delta)
		case "reasoning-delta":
			flushText()
			reasoningContent.WriteString(event.Text)
		case "tool-call", "toolCall":
			flushReasoning()
			flushText()
			input := map[string]any{}
			if event.Input != nil {
				if inputMap, ok := event.Input.(map[string]any); ok {
					input = inputMap
				} else if inputBytes, err := json.Marshal(event.Input); err == nil {
					_ = json.Unmarshal(inputBytes, &input)
				}
			}
			content = append(content, protocol.AnthropicContent{Type: "tool_use", ID: event.ToolCallID, Name: event.ToolName, Input: input})
		case "finish", "finish-step":
			if event.FinishReason != "" {
				finishReason = event.FinishReason
			}
			if event.TotalUsage != nil {
				usage.InputTokens = event.TotalUsage.InputTokens
				usage.OutputTokens = event.TotalUsage.OutputTokens
				usage.CachedInputTokens = event.TotalUsage.CachedInputTokens
			} else if event.Usage != nil {
				usage.InputTokens = event.Usage.InputTokens
				usage.OutputTokens = event.Usage.OutputTokens
				usage.CachedInputTokens = event.Usage.CachedInputTokens
			}
		case "error":
			if logger != nil {
				logger.Warn("CommandCode stream error event", map[string]any{
					"message": commandCodeEventMessage(event),
				})
			}
		}
	}

	flushReasoning()
	flushText()
	if usage.OutputTokens == 0 {
		if logger != nil {
			logger.Warn("Zero output tokens from upstream (non-streaming)", nil)
		}
		return nil, "", nil, apierror.NewAPIError(apierror.ErrorTypeRateLimit, "Empty response from upstream (zero output tokens)").WithCode(http.StatusTooManyRequests)
	}
	return content, finishReason, usage, nil
}

func commandCodeStreamToOpenAIWithIdleTimeout(reader io.Reader, model string, completionID string, created int64, logger *logging.Logger, idleTimeout time.Duration) (*protocol.OpenAIResponse, error) {
	lines := streaming.ScanLines(reader)
	var content strings.Builder
	var reasoningContent strings.Builder
	finishReason := "stop"
	usage := &protocol.Usage{}
	var toolCalls []protocol.ToolCall

	for {
		line, err := streaming.NextLine(lines, idleTimeout)
		if err != nil {
			if errors.Is(err, streaming.ErrStreamIdleTimeout) {
				count := consecutiveTimeouts.Add(1)
				message := timeoutMessage(count)
				return nil, apierror.NewAPIError(apierror.ErrorTypeRateLimit, message).WithCode(http.StatusTooManyRequests)
			}
			return nil, err
		}
		if line == "" {
			break
		}
		if line == "" || line == "[DONE]" {
			continue
		}

		var event streaming.CommandCodeEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}

		switch event.Type {
		case "text-delta":
			content.WriteString(streaming.TextDeltaContent(event))
		case "text":
			content.WriteString(event.Text)
		case "delta":
			content.WriteString(event.Delta)
		case "reasoning-delta":
			reasoningContent.WriteString(event.Text)
		case "tool-call", "toolCall":
			arguments := "{}"
			if event.Input != nil {
				if inputString, ok := event.Input.(string); ok {
					arguments = inputString
				} else if inputBytes, err := json.Marshal(event.Input); err == nil {
					arguments = string(inputBytes)
				}
			}
			toolCalls = append(toolCalls, protocol.ToolCall{
				ID:   event.ToolCallID,
				Type: "function",
				Function: protocol.FunctionCall{
					Name:      event.ToolName,
					Arguments: arguments,
				},
			})
		case "finish", "finish-step":
			if event.FinishReason != "" {
				finishReason = event.FinishReason
			}
			if event.TotalUsage != nil {
				usage.InputTokens = event.TotalUsage.InputTokens
				usage.OutputTokens = event.TotalUsage.OutputTokens
				usage.CachedInputTokens = event.TotalUsage.CachedInputTokens
			} else if event.Usage != nil {
				usage.InputTokens = event.Usage.InputTokens
				usage.OutputTokens = event.Usage.OutputTokens
				usage.CachedInputTokens = event.Usage.CachedInputTokens
			}
		case "error":
			if logger != nil {
				logger.Warn("CommandCode stream error event", map[string]any{
					"message": commandCodeEventMessage(event),
				})
			}
		}
	}

	if usage.OutputTokens == 0 {
		if logger != nil {
			logger.Warn("Zero output tokens from upstream (non-streaming)", nil)
		}
		return nil, apierror.NewAPIError(apierror.ErrorTypeRateLimit, "Empty response from upstream (zero output tokens)").WithCode(http.StatusTooManyRequests)
	}

	contentText := content.String()
	reasoningText := reasoningContent.String()
	message := &protocol.OpenAIMessage{Role: "assistant", Content: contentText}
	if contentText == "" && len(toolCalls) > 0 {
		message.Content = nil
	}
	if reasoningText != "" {
		message.ReasoningContent = reasoningText
	}
	if len(toolCalls) > 0 {
		message.ToolCalls = toolCalls
	}

	return &protocol.OpenAIResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: created,
		Model:   model,
		Choices: []protocol.OpenAIChoice{
			{
				Index:        0,
				Message:      message,
				FinishReason: finishReason,
			},
		},
		Usage: usage,
	}, nil
}

// commandCodeEventMessage returns a sanitized stream event message for logs.
func commandCodeEventMessage(event streaming.CommandCodeEvent) string {
	if event.Error != nil && event.Error.Message != "" {
		return event.Error.Message
	}
	return event.Message
}

// sendError sends an error response in OpenAI/Anthropic format
func sendError(w http.ResponseWriter, apiErr *apierror.APIError) {
	sendErrorResponse(w, apiErr, 0)
}

// sendErrorWithRetryAfter sends an error response with retry hints.
func sendErrorWithRetryAfter(w http.ResponseWriter, apiErr *apierror.APIError, retryAfter int) {
	sendErrorResponse(w, apiErr, retryAfter)
}

func sendErrorResponse(w http.ResponseWriter, apiErr *apierror.APIError, retryAfter int) {
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	}
	if apiErr.Code == 0 {
		apiErr.Code = http.StatusInternalServerError
	}
	w.WriteHeader(apiErr.Code)

	errorResp := map[string]any{
		"error": map[string]any{
			"type":    apiErr.Type,
			"message": apiErr.Message,
		},
	}
	if retryAfter > 0 {
		errorResp["retry_after"] = retryAfter
	}

	json.NewEncoder(w).Encode(errorResp)
}

// generateCompletionID generates a random completion ID
func generateCompletionID() string {
	return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
}

// generateMessageID generates a random message ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
