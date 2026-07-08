package http

import (
	"net/http"
	"testing"
)

func TestAPIKeyFromConfigExtractsConfigKey(t *testing.T) {
	apiKey, ok := ccAPIKeyFromConfig("prefix_user_config_key_suffix")
	if !ok {
		t.Fatal("expected API key to be extracted from config")
	}

	if got, want := apiKey, "user_config_key_suffix"; got != want {
		t.Fatalf("apiKey = %q, want %q", got, want)
	}
}

func TestAPIKeyFromConfigRejectsEmptyConfig(t *testing.T) {
	if apiKey, ok := ccAPIKeyFromConfig(""); ok {
		t.Fatalf("ccAPIKeyFromConfig() accepted %q, want rejection", apiKey)
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
