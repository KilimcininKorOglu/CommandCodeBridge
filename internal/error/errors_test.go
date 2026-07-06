package error

import "testing"

func TestMapStatusMatchesCommandCodeStatusMap(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		wantCode int
		wantType ErrorType
	}{
		{name: "bad request", status: 400, wantCode: 400, wantType: ErrorTypeInvalidRequest},
		{name: "unauthorized", status: 401, wantCode: 401, wantType: ErrorTypeAuth},
		{name: "payment required", status: 402, wantCode: 429, wantType: ErrorTypeRateLimit},
		{name: "forbidden", status: 403, wantCode: 401, wantType: ErrorTypeAuth},
		{name: "not found", status: 404, wantCode: 404, wantType: ErrorTypeNotFound},
		{name: "validation", status: 422, wantCode: 400, wantType: ErrorTypeInvalidRequest},
		{name: "rate limit", status: 429, wantCode: 429, wantType: ErrorTypeRateLimit},
		{name: "internal", status: 500, wantCode: 502, wantType: ErrorTypeUpstream},
		{name: "bad gateway", status: 502, wantCode: 502, wantType: ErrorTypeUpstream},
		{name: "unavailable", status: 503, wantCode: 503, wantType: ErrorTypeTemporarilyUnavailable},
		{name: "unknown", status: 418, wantCode: 502, wantType: ErrorTypeUpstream},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := MapStatus(tt.status, "sensitive upstream body")
			if apiErr.Code != tt.wantCode {
				t.Fatalf("Code = %d, want %d", apiErr.Code, tt.wantCode)
			}
			if apiErr.Type != tt.wantType {
				t.Fatalf("Type = %q, want %q", apiErr.Type, tt.wantType)
			}
			if apiErr.Message == "sensitive upstream body" {
				t.Fatal("MapStatus leaked raw upstream body")
			}
		})
	}
}
