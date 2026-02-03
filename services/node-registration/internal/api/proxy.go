package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"node-registration/internal/websocket"
)

// ProxyHandler handles internal proxy routing requests
type ProxyHandler struct {
	proxyManager *websocket.ProxyManager
	logger       *logrus.Entry
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(pm *websocket.ProxyManager, logger *logrus.Entry) *ProxyHandler {
	return &ProxyHandler{
		proxyManager: pm,
		logger:       logger.WithField("component", "proxy-api"),
	}
}

// ProxyRequestPayload is the request body for proxy requests
type ProxyRequestPayload struct {
	NodeID    string            `json:"node_id"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	Method    string            `json:"method,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Body      string            `json:"body,omitempty"` // Base64 encoded
	TimeoutMs int               `json:"timeout_ms"`
}

// ProxyResponsePayload is the response body for proxy requests
type ProxyResponsePayload struct {
	Success    bool              `json:"success"`
	StatusCode int               `json:"status_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"` // Base64 encoded
	Error      string            `json:"error,omitempty"`
	BytesRead  int64             `json:"bytes_read"`
	BytesWrite int64             `json:"bytes_write"`
	LatencyMs  int64             `json:"latency_ms"`
}

// HandleProxyRequest handles HTTP/TCP proxy requests
func (h *ProxyHandler) HandleProxyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload ProxyRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.NodeID == "" || payload.Host == "" || payload.Port == "" {
		http.Error(w, "Missing required fields: node_id, host, port", http.StatusBadRequest)
		return
	}

	h.logger.Infof("Proxy request for %s:%s via node %s", payload.Host, payload.Port, payload.NodeID)

	// Set default timeout
	timeout := payload.TimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}

	// Create proxy request
	req := &websocket.ProxyRequest{
		Host:    payload.Host,
		Port:    payload.Port,
		Method:  payload.Method,
		URL:     payload.URL,
		Headers: payload.Headers,
		Body:    payload.Body,
		Timeout: timeout,
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeout+5000)*time.Millisecond)
	defer cancel()

	// Send request through node
	resp, err := h.proxyManager.SendProxyRequest(ctx, payload.NodeID, req)
	if err != nil {
		h.logger.Errorf("Proxy request failed: %v", err)
		
		response := ProxyResponsePayload{
			Success: false,
			Error:   err.Error(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return response
	response := ProxyResponsePayload{
		Success:    resp.Success,
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
		Error:      resp.Error,
		BytesRead:  resp.BytesRead,
		BytesWrite: resp.BytesWrite,
		LatencyMs:  resp.LatencyMs,
	}

	w.Header().Set("Content-Type", "application/json")
	if !resp.Success {
		w.WriteHeader(http.StatusBadGateway)
	}
	json.NewEncoder(w).Encode(response)
}

// HandleTunnelOpen handles TCP tunnel open requests (for CONNECT)
func (h *ProxyHandler) HandleTunnelOpen(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement WebSocket upgrade for bidirectional tunnel
	// This will be needed for HTTPS CONNECT tunneling
	http.Error(w, "Not implemented yet", http.StatusNotImplemented)
}
