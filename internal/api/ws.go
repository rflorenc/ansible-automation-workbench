package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamJobLogs streams job log lines over WebSocket.
func (s *Server) StreamJobLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job := s.Jobs.Get(id)
	if job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	offset := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lines := job.LogsSince(offset)
			for _, line := range lines {
				if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
					return
				}
				offset++
			}
			// If job is done and we've sent everything, close
			if (job.Status == "completed" || job.Status == "failed") && len(lines) == 0 {
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, job.Status))
				return
			}
		}
	}
}
