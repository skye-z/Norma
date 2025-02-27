package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"norma/internal/runtime"
	"norma/internal/runtime/bus"
)

type RuntimeHandler struct {
	bus       *bus.Bus
	inspector *runtime.Inspector
}

func NewRuntimeHandler(b *bus.Bus, inspector *runtime.Inspector) *RuntimeHandler {
	return &RuntimeHandler{bus: b, inspector: inspector}
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
	mux.HandleFunc("/runtime/diagnostics", h.handleDiagnostics)
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

func (h *RuntimeHandler) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if h.inspector == nil {
		writeErr(w, http.StatusNotImplemented, "diagnostics unavailable")
		return
	}
	snapshot := h.inspector.Snapshot()
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit > 0 && len(snapshot.Events) > limit {
		snapshot.Events = snapshot.Events[len(snapshot.Events)-limit:]
	}
	writeJSON(w, http.StatusOK, snapshot)
}
