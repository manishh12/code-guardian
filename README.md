# Code Guardian

Go backend for automated GitHub PR reviews: multi-agent loop, RAG guidelines, token/latency/cost dashboard.

## Order of work (do this in order)

1. Install tools  
2. Run locally  
3. Run tests (must pass)  
4. Deploy  
5. Point GitHub webhooks at the live URL  

Do **not** deploy if tests fail.

---

## 1. Install (all free)

| Tool | Why |
| --- | --- |
| [Go 1.22+](https://go.dev/dl/) | Backend + tests. Restart the terminal after install. |
| [Docker Desktop](https://www.docker.com/products/docker-desktop/) | Postgres with pgvector |
| [Git](https://git-scm.com/download/win) | Push for deploy + CI |
| Optional: [GoLand](https://www.jetbrains.com/go/) | Open this folder as a Go module |
| Optional: [Groq key](https://console.groq.com) | Real LLM reviews (still free tier) |

Check:

```powershell
go version
docker version
git --version
```

---

## 2. Run locally

From `C:\Users\HP\OneDrive\Desktop\code-guardian`:

```powershell
copy .env.example .env
docker compose up --build
```

This starts Postgres **and** the API, then seeds guidelines.

- Health: http://localhost:8787/health  
- Demo review: `POST http://localhost:8787/webhooks/demo`  
- Metrics: http://localhost:8787/api/metrics  

PowerShell demo:

```powershell
Invoke-RestMethod -Method POST http://localhost:8787/webhooks/demo | ConvertTo-Json -Depth 6
Invoke-RestMethod http://localhost:8787/api/metrics | ConvertTo-Json -Depth 6
```

Without Docker for the API (DB still needed):

```powershell
docker compose up -d postgres
go mod tidy
go run ./cmd/seed
go run ./cmd/server
```

Optional dashboard UI:

```powershell
cd web
npm install
npm run dev
```

Then open http://localhost:5173 and click **Run demo review**.

Default `.env` uses `LLM_PROVIDER=mock` (no API key). For a real free model, set `GROQ_API_KEY` and `LLM_PROVIDER=groq`, then recreate the API container.

---

## 3. Test (required before deploy)

```powershell
go mod tidy
go test ./...
```

You should see `PASS` for:

- HMAC webhook signatures  
- local embeddings  
- mock LLM  
- health + ping webhook  
- cost equivalent math  

GitHub Actions (`.github/workflows/ci.yml`) runs the same tests on every push.

GoLand: **Run → Run 'go test ./...'** or right-click `internal` → **Run Tests**.

---

## 4. Deploy (after tests pass)

### A. Supabase database

1. Create a free project at https://supabase.com.
2. Open **SQL Editor** and run `CREATE EXTENSION IF NOT EXISTS vector;`.
3. Open **Project Settings → Database** and copy the direct connection string. Replace its password placeholder.
4. Use that connection string as `DATABASE_URL`.

### B. Render API + dashboard

The production Docker image builds React and copies `web/dist` into the Go service. The dashboard is served at `/`; no second frontend service is needed.

1. Go to https://render.com and connect the GitHub repository.
2. Choose **New → Blueprint** and select this repository. Render reads `render.yaml`.
3. Enter the required environment variables:

   - `DATABASE_URL` — Supabase direct PostgreSQL connection string
   - `LLM_PROVIDER=groq`
   - `GROQ_API_KEY`
   - `GROQ_MODEL=openai/gpt-oss-20b`
   - `GITHUB_WEBHOOK_SECRET` — pick a long random string  
   - `GITHUB_TOKEN` — PAT that can read PRs and comment  

4. Deploy.
5. Open `https://YOUR-SERVICE.onrender.com/` for the dashboard.
6. Confirm `https://YOUR-SERVICE.onrender.com/health` returns `"ok": true`.
7. Hit `POST https://YOUR-SERVICE.onrender.com/webhooks/demo` once.

### C. GitHub webhook (live PRs)

On the repo you want reviewed:

1. Settings → Webhooks → Add webhook  
2. Payload URL: `https://YOUR-SERVICE.onrender.com/webhooks/github`  
3. Content type: `application/json`  
4. Secret: same as `GITHUB_WEBHOOK_SECRET`  
5. Events: **Pull requests**  
6. Open a test PR. You should get one aggregated review comment.

---

## 5. Pass / fail checklist

- [ ] `go test ./...` is green  
- [ ] `/health` works locally  
- [ ] `/webhooks/demo` returns markdown + token metrics  
- [ ] `/health` works on the deployed URL  
- [ ] `/webhooks/demo` works on the deployed URL  
- [ ] GitHub ping webhook is green  
- [ ] A real PR gets a comment (needs `GITHUB_TOKEN`)  

## Layout

```
cmd/server          API
cmd/seed            Seed RAG guidelines
internal/           agents, llm, rag, github, metrics, api
guidelines/         markdown for RAG
web/                React dashboard, bundled into production
Dockerfile          production image
docker-compose.yml  local Postgres + API
.github/workflows   CI tests
```
