# Polentrix

Local ChatGPT-style chatbot powered by [Ollama](https://ollama.com/), with a Vue UI and a Go streaming API.

## Stack

| Piece | Role |
|-------|------|
| **web** (Vue 3 + Vite) | Chat UI — chat list, new chat, streaming messages |
| **api** (Go) | Thin proxy — SSE streaming from Ollama |
| **ollama** | Local LLM runtime |
| **ollama-init** | Pulls the default model on first start |

No database and no worker in v1. Chats live in the browser (`localStorage`). Add SQLite/Postgres later when you want sync or multi-user; add a worker only for async jobs (embeddings, file ingest, etc.).

## Requirements

- Docker + Docker Compose
- ~1GB+ free RAM for `qwen2.5:0.5b` (more for larger tags)

## Quick start

```bash
cp .env.example .env   # optional
docker compose up --build
```

On first boot, `ollama-init` pulls `qwen2.5:0.5b` (can take a few minutes).

Then open:

- UI: http://localhost:3000
- API health: http://localhost:8080/health
- Ollama: http://localhost:11434

## Models

Default: `qwen2.5:0.5b` (lightest smoke test).

| Goal | Set in `.env` |
|------|----------------|
| Fastest test | `OLLAMA_MODEL=qwen2.5:0.5b` |
| Better quality, still light | `OLLAMA_MODEL=qwen2.5:1.5b` |
| Comfortable next step | `OLLAMA_MODEL=qwen2.5:3b` |

After changing the model:

```bash
docker compose up -d
# or pull manually:
docker compose exec ollama ollama pull qwen2.5:1.5b
```

Then restart the API so it uses the new default:

```bash
docker compose up -d api
```

## Local development (without rebuilding web)

1. Start Ollama + API:

```bash
docker compose up ollama ollama-init api
```

2. In another terminal:

```bash
cd frontend
npm install
npm run dev
```

Vite serves http://localhost:5173 and proxies `/api` and `/health` to `localhost:8080`.

Run the API on the host instead:

```bash
cd backend
OLLAMA_BASE_URL=http://localhost:11434 OLLAMA_MODEL=qwen2.5:0.5b go run ./cmd/api
```

## API

- `GET /health` — status + configured model
- `GET /api/models` — models known to Ollama + default
- `POST /api/chat` — body `{ "messages": [{ "role", "content" }] }`, response is SSE:

```
data: {"content":"Hello","done":false,"role":"assistant"}

data: [DONE]
```

## Project layout

```
polentrix/
  docker-compose.yml
  .env.example
  backend/     # Go API
  frontend/    # Vue SPA
```

## Later

- Persist chats in SQLite/Postgres via the Go API
- Auth / multi-user
- Worker for RAG or file processing
- Larger Qwen (or other) models as your hardware allows
