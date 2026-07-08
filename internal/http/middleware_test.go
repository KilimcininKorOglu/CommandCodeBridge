package http

import (
	"net/http"
	"testing"
)

func TestAPIKeyFromRequestExtractsBearerKey(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer token_user_abc-123_extra")

	apiKey, ok := ccAPIKeyFromRequest(headers, "")
	if !ok {
		t.Fatal("expected API key to be extracted from Authorization header")
	}

	if got, want := apiKey, "user_abc-123_extra"; got != want {
		t.Fatalf("apiKey = %q, want %q", got, want)
	}
}

func TestAPIKeyFromRequestUsesConfigFallbackWithoutHeader(t *testing.T) {
	headers := http.Header{}

	apiKey, ok := ccAPIKeyFromRequest(headers, "user_config_key")
	if !ok {
		t.Fatal("expected API key to be extracted from config fallback")
	}

	if got, want := apiKey, "user_config_key"; got != want {
		t.Fatalf("apiKey = %q, want %q", got, want)
	}
}

func TestAPIKeyFromRequestUsesConfigFallbackForInvalidBearer(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer sk-invalid")

	apiKey, ok := ccAPIKeyFromRequest(headers, "user_config_key")
	if !ok {
		t.Fatal("expected API key to be extracted from config fallback")
	}
	if got, want := apiKey, "user_config_key"; got != want {
		t.Fatalf("apiKey = %q, want %q", got, want)
	}
}

func TestProxyTokenFromRequestAcceptsBearerProxyToken(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test")

	if !proxyTokenFromRequest(headers, "test") {
		t.Fatal("expected proxy token to be accepted from Authorization header")
	}
}

func TestProxyTokenFromRequestAcceptsProxyTokenHeader(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Proxy-Token", "test")

	if !proxyTokenFromRequest(headers, "test") {
		t.Fatal("expected proxy token to be accepted from X-Proxy-Token header")
	}
}

func TestProxyTokenFromRequestRejectsMissingToken(t *testing.T) {
	headers := http.Header{}

	if proxyTokenFromRequest(headers, "test") {
		t.Fatal("expected missing proxy token to be rejected")
	}
}

func TestAPIKeyFromRequestUsesConfigFallbackForProxyTokenBearer(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test")

	apiKey, ok := ccAPIKeyFromRequest(headers, "user_config_key")
	if !ok {
		t.Fatal("expected API key to be extracted from config fallback")
	}
	if got, want := apiKey, "user_config_key"; got != want {
		t.Fatalf("apiKey = %q, want %q", got, want)
	}
}

func TestAPIKeyFromRequestRejectsInvalidBearerWithoutConfigFallback(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer test")

	if apiKey, ok := ccAPIKeyFromRequest(headers, ""); ok {
		t.Fatalf("ccAPIKeyFromRequest() accepted %q, want rejection", apiKey)
	}
}

func TestProxyTokenFromRequestAcceptsXAPIKeyHeader(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-api-key", "my-proxy-token")

	if !proxyTokenFromRequest(headers, "my-proxy-token") {
		t.Fatal("expected proxy token to be accepted from x-api-key header")
	}
}

func TestProxyTokenFromRequestAcceptsXAPIKeyWhenAuthorizationHasCommandCodeKey(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer user_upstream_key")
	headers.Set("x-api-key", "my-proxy-token")

	if !proxyTokenFromRequest(headers, "my-proxy-token") {
		t.Fatal("expected x-api-key proxy token to be accepted when Authorization contains upstream key")
	}
}

func TestProxyTokenFromRequestRejectsWrongXAPIKey(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-api-key", "wrong-token")

	if proxyTokenFromRequest(headers, "my-proxy-token") {
		t.Fatal("expected wrong x-api-key proxy token to be rejected")
	}
}
