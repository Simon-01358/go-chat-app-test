package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

//go:embed static
var staticFiles embed.FS

const (
	maxHistory = 50
	maxName    = 24
	maxText    = 500
)

// Message is one chat message.
type Message struct {
	Name string    `json:"name"`
	Text string    `json:"text"`
	Time time.Time `json:"time"`
}

// Hub keeps the message history and all connected browsers.
type Hub struct {
	mu      sync.Mutex
	history []Message
	clients map[chan string]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan string]struct{})}
}

// subscribe registers a new browser and returns its channel plus the history so far.
func (h *Hub) subscribe() (chan string, []Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan string, 32)
	h.clients[ch] = struct{}{}
	past := make([]Message, len(h.history))
	copy(past, h.history)
	h.broadcastLocked(presenceEvent(len(h.clients)))
	return ch, past
}

func (h *Hub) unsubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, ch)
	close(ch)
	h.broadcastLocked(presenceEvent(len(h.clients)))
}

// publish stores a message and sends it to everyone.
func (h *Hub) publish(m Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history = append(h.history, m)
	if len(h.history) > maxHistory {
		h.history = h.history[len(h.history)-maxHistory:]
	}
	data, _ := json.Marshal(m)
	h.broadcastLocked(sseEvent("message", string(data)))
}

func (h *Hub) broadcastLocked(event string) {
	for ch := range h.clients {
		select {
		case ch <- event:
		default: // slow client: drop the event rather than block everyone
		}
	}
}

func sseEvent(name, data string) string {
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, data)
}

func presenceEvent(n int) string {
	return sseEvent("presence", fmt.Sprintf(`{"online":%d}`, n))
}

func main() {
	hub := NewHub()
	mux := http.NewServeMux()

	// Frontend
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// Health check for Render
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Live stream of messages (Server-Sent Events)
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // stop proxies from buffering the stream

		ch, past := hub.subscribe()
		defer hub.unsubscribe(ch)

		// Send existing history first
		for _, m := range past {
			data, _ := json.Marshal(m)
			fmt.Fprint(w, sseEvent("message", string(data)))
		}
		flusher.Flush()

		keepAlive := time.NewTicker(20 * time.Second)
		defer keepAlive.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case ev, open := <-ch:
				if !open {
					return
				}
				fmt.Fprint(w, ev)
				flusher.Flush()
			case <-keepAlive.C:
				fmt.Fprint(w, ": ping\n\n") // comment line keeps the connection open
				flusher.Flush()
			}
		}
	})

	// Send a message
	mux.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var in struct {
			Name string `json:"name"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(in.Name)
		text := strings.TrimSpace(in.Text)
		if name == "" {
			name = "Anonymous"
		}
		if text == "" {
			http.Error(w, "text is required", http.StatusBadRequest)
			return
		}
		if utf8.RuneCountInString(name) > maxName {
			name = string([]rune(name)[:maxName])
		}
		if utf8.RuneCountInString(text) > maxText {
			text = string([]rune(text)[:maxText])
		}
		hub.publish(Message{Name: name, Text: text, Time: time.Now().UTC()})
		w.WriteHeader(http.StatusNoContent)
	})

	port := os.Getenv("PORT") // Render sets this
	if port == "" {
		port = "8080"
	}
	log.Printf("Chat server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
