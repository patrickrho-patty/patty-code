package openaiapi

import (
	"net/http"
	"strings"
)

// Header helpers shared by the model-fetch client and the OpenAI-compatible
// chat client. They live in this leaf package so the chat-client packages can
// be compiled out of non-public builds (ADR G4) without forking the logic.

// CleanCustomHeaders drops empty and reserved names/values from a user
// supplied header map.
func CleanCustomHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for rawName, rawValue := range in {
		name := strings.TrimSpace(rawName)
		value := strings.TrimSpace(rawValue)
		if name == "" || value == "" || reservedCustomHeader(name) {
			continue
		}
		out[name] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplyCustomHeaders sets cleaned custom headers on h.
func ApplyCustomHeaders(h http.Header, headers map[string]string) {
	for name, value := range CleanCustomHeaders(headers) {
		h.Set(name, value)
	}
}

// ApplyAPIKeyHeader sets the auth header for an OpenAI-compatible request:
// MiMo endpoints take an api-key header, everything else a bearer token.
func ApplyAPIKeyHeader(h http.Header, baseURL, apiKey string) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return
	}
	if IsMiMoEndpoint(baseURL) {
		h.Set("api-key", apiKey)
		return
	}
	h.Set("Authorization", "Bearer "+apiKey)
}

func reservedCustomHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "content-type", "accept", "host":
		return true
	default:
		return false
	}
}
