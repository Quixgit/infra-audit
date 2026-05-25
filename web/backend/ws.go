package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Non-browser clients (e.g. server-side, curl) don't send Origin — allow.
			return true
		}
		allowed := strings.Split(envOr("CORS_ORIGINS", "http://localhost:3000,http://frontend:3000"), ",")
		for _, o := range allowed {
			if strings.TrimSpace(o) == origin {
				return true
			}
		}
		return false
	},
}

type wsClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) send(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.WriteMessage(websocket.TextMessage, b)
}

type hub struct {
	mu      sync.RWMutex
	clients map[string][]*wsClient
}

func newHub() *hub {
	return &hub{clients: make(map[string][]*wsClient)}
}

func (h *hub) subscribe(jobID string, c *wsClient) {
	h.mu.Lock()
	h.clients[jobID] = append(h.clients[jobID], c)
	h.mu.Unlock()
}

func (h *hub) unsubscribe(jobID string, c *wsClient) {
	h.mu.Lock()
	list := h.clients[jobID]
	for i, cl := range list {
		if cl == c {
			h.clients[jobID] = append(list[:i], list[i+1:]...)
			break
		}
	}
	h.mu.Unlock()
}

func (h *hub) broadcast(jobID string, msg wsMessage) {
	h.mu.RLock()
	list := h.clients[jobID]
	h.mu.RUnlock()
	for _, c := range list {
		c.send(msg)
	}
}

func (srv *server) handleJobWS(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")

	// Accept token from Authorization header or ?token= query param
	tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	c, err := validateAccessToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	job, err := srv.getJob(r.Context(), jobID)
	if err != nil || job.TenantID != c.TenantID {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade: %v", err)
		return
	}
	defer conn.Close()

	client := &wsClient{conn: conn}
	srv.hub.subscribe(jobID, client)
	defer srv.hub.unsubscribe(jobID, client)

	// Send current state immediately
	msg := jobToWSMessage(job)
	client.send(msg)

	// Keep open until client disconnects
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func jobToWSMessage(job AuditJob) wsMessage {
	msg := wsMessage{
		JobID:       job.ID,
		Status:      job.Status,
		ProgressMsg: job.ProgressMsg,
		ErrorMsg:    job.ErrorMsg,
		FinishedAt:  job.FinishedAt,
	}
	if job.Status == "done" || job.Status == "failed" {
		msg.Findings = &struct {
			Critical int `json:"critical"`
			High     int `json:"high"`
			Medium   int `json:"medium"`
			Low      int `json:"low"`
		}{
			Critical: job.FindingsCritical,
			High:     job.FindingsHigh,
			Medium:   job.FindingsMedium,
			Low:      job.FindingsLow,
		}
	}
	return msg
}
