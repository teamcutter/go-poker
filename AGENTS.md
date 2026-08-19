# GoPoker — Development Guide

GoPoker is a Telegram Mini App Texas Hold'em game. Monorepo with:

- `backend/` — Go 1.26, Echo, PostgreSQL (pgx/v5), Redis. DDD / layered architecture.
- `frontend/` — Vite + React 18 + TypeScript. Telegram Mini App.
- `docker-compose.yml` — postgres, redis, backend, frontend.

## Commands

### Backend
- Build: `cd backend && go build ./...`
- Run tests: `cd backend && go test ./...`
- Format: `cd backend && gofmt -w .`
- Lint/vet: `cd backend && go vet ./...`
- Run locally (needs postgres/redis): `cd backend && go run ./cmd/server`

### Frontend
- Install: `cd frontend && npm install`
- Dev: `cd frontend && npm run dev` (proxies `/api` and `/healthz` to `localhost:8080`)
- Build: `cd frontend && npm run build`
- Type-check: `cd frontend && npm run typecheck`

### Full stack
- `cp .env.example .env`, then `docker compose up --build`
- Backend API: `http://localhost:8080`, health: `GET /healthz`
- Frontend: `http://localhost:5173`

## Architecture

DDD layering: domain (pure rules + repository interfaces) → application (use cases) → infrastructure (postgres/redis/session/telegram adapters) → transport/http (Echo handlers).

- Domain repos are interfaces; infra `internal/infrastructure/postgres` implements them and is tx-aware via `store.Manager.Run` / `store.WithTx` / `store.From`.
- Application services depend on the interfaces, never on infra.
- `internal/transport/http` maps domain sentinels to HTTP statuses via `mapError`; response envelope `{"data":…}` / `{"error":{"code","message"}}`.

Postgres holds **only** the `users` table, for authentication. All poker state — tables, seats, hands, bankrolls — lives in memory in `application/poker.Service` and is lost on restart.

### Key conventions
- Never trust client state: Telegram initData is validated server-side (HMAC), then a JWT session is issued.
- `index.html` must load `telegram-web-app.js`; without it `window.Telegram` is absent and every client silently falls back to `VITE_DEV_TOKEN`, authenticating as one shared account.
- `VITE_DEV_TOKEN` is gated behind `import.meta.env.DEV` so it cannot reach a production bundle, and `.env` is in `.dockerignore`.
- The response envelope and error codes must stay stable (the frontend depends on them).
- `SESSION_SECRET` signs JWTs and is freely rotatable: rotating it only logs everyone out. It is the sole secret in the auth path.
- Only `Claims.PublicID` may be published — a random UUID on the user row, carried in the token as `pid` so no lookup is needed per request. `Claims.UserID` is the raw Telegram id, the Postgres primary key, and must never reach a client or every opponent gains a permanent handle on the account.

## API surface
`POST /api/auth/telegram`, `GET /healthz`.

Poker: `POST|GET /api/tables`, `GET /api/tables/:code`, `POST /api/tables/:code/{join,leave,start}`, `GET /api/poker/wallet`, `POST /api/poker/wallet/topup`, `GET /api/ws/tables/:code` (websocket).

### Poker rules of the house
- Chips live in an in-memory bankroll (`application/poker.Service.wallets`); joining deducts a buy-in, leaving returns the stack.
- A player with an empty stack sits out: `Table.StartHand` deals only to seats with `Stack > 0` and needs two of them.
- `TopUp` grants a free stack only when the bankroll is empty and the player has no chips at any table, and broadcasts the new balance to every table they sit at.
- Finished hands stay in `PhaseComplete` with hole cards intact so the client can reveal them; `StartHand` clears and redeals.
- Blinds follow the standard: heads-up the button posts the small blind and acts first preflop; three-handed and up the small blind is left of the button. Post-flop the action always opens left of the button, so the button acts last.
- Empty tables are closed by `Service.ReapIdle` after `IdleGrace` (30s), driven by `RunReaper` from `cmd/server`.
- A seat is only safe while a websocket is connected. `touchSeatLocked` starts an away clock whenever a seat is taken or rejoined without a live socket; `ReapIdle` seats the player out after `AwayGrace` (30s) and refunds their stack. Presence is a socket count, so a reconnect racing the old socket's teardown keeps the seat.
- The acting player has `TurnLimit` (30s). `EnforceTurns` checks when free, otherwise folds. The clock is keyed on seat **and** phase, and `turn_ms_left` is sent as a remaining duration so clients are immune to clock skew.
- Only `Table.NextStarterSeat()` may deal; the right rotates with the button each round (`ErrNotYourDeal`).
- **`notify` must only be called with `s.mu` released.** The websocket broadcaster builds a per-viewer DTO via `Bankroll`/`TurnRemaining`, which re-enter the service; notifying under the lock self-deadlocks on the non-reentrant mutex. Mutators use deferred LIFO (`defer notify` registered before `defer Unlock`) to guarantee this.

## Requirements for changes
- Keep Go files **comment-free** (this is an intentional project rule).
- Run `gofmt`, `go vet`, and `go test ./...` before finishing backend work.
- Frontend must pass `npm run typecheck` (strict TS, no unused locals).
- Migrations are append-only. `postgres.Migrate` is a hand-rolled forward-only runner keyed by filename, so it ignores `schema_migrations` rows whose files no longer exist.
