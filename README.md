# Go Chat

A small real-time chat room written in Go. Standard library only, no dependencies.

- **Backend:** `main.go` (Go `net/http`)
- **Frontend:** `static/` (HTML, CSS, JavaScript), embedded into the Go binary
- **Real-time:** Server-Sent Events (server to browser) + a normal POST request (browser to server)
- **Data:** the last 50 messages are kept in memory (lost on restart)

## Endpoints

| Method | Path            | What it does                                        |
|--------|-----------------|-----------------------------------------------------|
| GET    | `/`             | The chat page                                       |
| GET    | `/events`       | Live stream: past messages, new messages, online count |
| POST   | `/api/messages` | Send a message: `{ "name": "...", "text": "..." }`   |
| GET    | `/health`       | Health check                                        |

## Run locally

```bash
go run .
# open http://localhost:8080 in two browser tabs
```

## Deploy on Render

1. Push this folder to a GitHub repository.
2. In Render: **New +** > **Web Service** > connect the repo.
3. Settings (auto-filled if you use **New + > Blueprint** with `render.yaml`):
   - Runtime: **Go**
   - Build Command: `go build -o app .`
   - Start Command: `./app`
   - Health Check Path: `/health`
   - Plan: **Free**
4. Click **Create Web Service**, then open the `.onrender.com` URL in two tabs and chat.

Free instances sleep after ~15 minutes of inactivity; the first request afterwards can take 30-60 seconds, and chat history is cleared when the instance restarts.
