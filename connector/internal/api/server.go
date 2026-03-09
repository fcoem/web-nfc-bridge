package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"nfc-tool/connector/internal/auth"
	"nfc-tool/connector/internal/bridge"

	"github.com/gorilla/websocket"
)

type Server struct {
	service        *bridge.Service
	allowedOrigins []string
	sharedSecret   string
	version        string
	buildTime      string
	upgrader       websocket.Upgrader
	clients        map[*websocket.Conn]struct{}
	mu             sync.Mutex
}

func NewServer(service *bridge.Service, allowedOrigins []string, sharedSecret string, version string, buildTime string) *Server {
	normalizedOrigins := make([]string, 0, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			normalizedOrigins = append(normalizedOrigins, trimmed)
		}
	}

	server := &Server{
		service:        service,
		allowedOrigins: normalizedOrigins,
		sharedSecret:   sharedSecret,
		version:        version,
		buildTime:      buildTime,
		clients:        map[*websocket.Conn]struct{}{},
	}

	server.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return server.isOriginAllowed(r.Header.Get("Origin"))
		},
	}

	go server.fanoutEvents()
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/readers", s.withAuth(s.handleReaders, "all"))
	mux.HandleFunc("/session/connect", s.withAuth(s.handleConnectSession, "all"))
	mux.HandleFunc("/card/read", s.withAuth(s.handleReadCard, "read"))
	mux.HandleFunc("/card/write", s.withAuth(s.handleWriteCard, "write"))
	mux.HandleFunc("/events", s.handleEvents)
	return mux
}

func (s *Server) fanoutEvents() {
	for event := range s.service.Events() {
		s.mu.Lock()
		for client := range s.clients {
			client.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := client.WriteJSON(event); err != nil {
				client.Close()
				delete(s.clients, client)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		s.applyCORS(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.applyCORS(w, r)
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": s.version,
		"buildTime": s.buildTime,
		"driver":  s.service.DriverName(),
		"details": s.service.Health(r.Context()),
	})
}

func (s *Server) handleReaders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	readers, err := s.service.Readers(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{"readers": readers})
}

func (s *Server) handleConnectSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		ReaderName string `json:"readerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, context.Canceled) && err.Error() != "EOF" {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid connect request"})
		return
	}

	session, err := s.service.OpenSession(r.Context(), body.ReaderName)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleReadCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		SessionID string `json:"sessionId"`
		Operation string `json:"operation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid read request"})
		return
	}

	if body.Operation == "" {
		body.Operation = "summary"
	}

	result, err := s.service.Read(r.Context(), body.SessionID, body.Operation)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleWriteCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		SessionID string         `json:"sessionId"`
		Operation string         `json:"operation"`
		Payload   map[string]any `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid write request"})
		return
	}

	if body.Operation == "" {
		body.Operation = bridge.NDEFWriteOperation
	}

	result, err := s.service.Write(r.Context(), body.SessionID, body.Operation, body.Payload)
	if err != nil {
		s.writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	status := http.StatusOK
	if !result.Accepted {
		status = http.StatusConflict
	}
	
	s.writeJSON(w, status, result)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	origin := r.Header.Get("Origin")
	if !s.isOriginAllowed(origin) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	token := r.URL.Query().Get("token")
	if _, err := auth.VerifyTicket(token, s.sharedSecret, origin, "events"); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		s.mu.Unlock()
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) withAuth(next http.HandlerFunc, scope string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			s.applyCORS(w, r)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		origin := r.Header.Get("Origin")
		if !s.isOriginAllowed(origin) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}

		token := r.Header.Get("X-Bridge-Token")
		if _, err := auth.VerifyTicket(token, s.sharedSecret, origin, scope); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		s.applyCORS(w, r)
		next(w, r)
	}
}

func (s *Server) applyCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" || !s.isOriginAllowed(origin) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Bridge-Token")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func (s *Server) isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowedOrigin := range s.allowedOrigins {
		if allowedOrigin == origin {
			return true
		}
		if matchesOriginPattern(allowedOrigin, origin) {
			return true
		}
	}

	return false
}

func matchesOriginPattern(pattern string, origin string) bool {
	if !strings.Contains(pattern, ":*") {
		return false
	}

	patternURL, err := url.Parse(strings.Replace(pattern, ":*", "", 1))
	if err != nil {
		return false
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	if patternURL.Scheme != originURL.Scheme {
		return false
	}

	if !sameLoopbackHost(patternURL.Hostname(), originURL.Hostname()) {
		return false
	}

	return originURL.Port() != ""
}

func sameLoopbackHost(expected string, actual string) bool {
	if expected == actual {
		return true
	}

	loopbackHosts := map[string]struct{}{
		"localhost": {},
		"127.0.0.1": {},
		"::1":       {},
	}

	_, expectedLoopback := loopbackHosts[expected]
	_, actualLoopback := loopbackHosts[actual]
	return expectedLoopback && actualLoopback
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}