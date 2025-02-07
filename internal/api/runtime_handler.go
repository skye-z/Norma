package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"norma/internal/runtime/bus"
)

type RuntimeHandler struct {
	bus *bus.Bus
}

func NewRuntimeHandler(b *bus.Bus) *RuntimeHandler {
	return &RuntimeHandler{bus: b}
}

type publishReq struct {
	Name      string `json:"name"`
	SessionID string `json:"session_id"`
	TraceID   string `json:"trace_id"`
	Payload   any    `json:"payload"`
}

func (h *RuntimeHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/runtime/publish", h.handlePublish)
	mux.HandleFunc("/runtime/events", h.handleEvents)
}

func (h *RuntimeHandler) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req publishReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}

	h.bus.Publish(bus.Event{
		Name:      req.Name,
		SessionID: req.SessionID,
		TraceID:   req.TraceID,
		Payload:   req.Payload,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *RuntimeHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeErr(w, http.StatusBadRequest, "name query required")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "stream unsupported")
		return
	}

	// Flush once so clients can establish SSE before first event.
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	eventCh := make(chan bus.Event, 16)
	unsub := h.bus.Subscribe(name, func(evt bus.Event) {
		select {
		case eventCh <- evt:
		default:
		}
	})
	defer unsub()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-eventCh:
			b, _ := json.Marshal(evt)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
