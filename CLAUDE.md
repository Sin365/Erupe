# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Erupe is a Go server emulator for Monster Hunter Frontier, a shut-down MMORPG. It handles authentication, world selection, and gameplay in a single binary running four TCP/HTTP servers. Go 1.25+ required.

## Build & Test Commands

```bash
go build -o erupe-ce                    # Build server
go build -o protbot ./cmd/protbot/      # Build protocol bot
go test -race ./... -timeout=10m        # Run tests (race detection mandatory)
go test -v ./server/channelserver/...   # Test one package
go test -run TestHandleMsg ./server/channelserver/...  # Single test
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out  # Coverage (CI requires ≥50%)
gofmt -w .                              # Format
golangci-lint run ./...                 # Lint (v2 standard preset, must pass CI)
```

Docker (from `docker/`):
```bash
docker compose up db pgadmin            # PostgreSQL + pgAdmin (port 5050)
docker compose up server                # Erupe (after DB is healthy)
```

## Architecture

### Four-Server Model (single binary, orchestrated from `main.go`)

```
Client ←[Blowfish TCP]→ Sign Server (53312)      → Authentication, sessions
                       → Entrance Server (53310)  → Server list, character select
                       → Channel Servers (54001+) → Gameplay, quests, multiplayer
                       → API Server (8080)        → REST API (/health, /version, V2 sign)
```

Each server is in its own package under `server/`. The channel server is by far the largest (~200 files).

### Channel Server Packet Flow

1. `network/crypt_conn.go` decrypts TCP stream (Blowfish)
2. `network/mhfpacket/` deserializes binary packet into typed struct (~453 packet types, one file each)
3. `handlers_table.go` dispatches via `buildHandlerTable()` (~200+ `PacketID → handlerFunc` entries)
4. Handler in appropriate `handlers_*.go` processes it (organized by game system)

Handler signature: `func(s *Session, p mhfpacket.MHFPacket)`

### Layered Architecture

```
handlers_*.go  →  svc_*.go (service layer)  →  repo_*.go (data access)
                                                    ↓
                                              repo_interfaces.go (21 interfaces)
                                                    ↓
                                              repo_mocks_test.go (test doubles)
```

- **Handlers**: Parse packets, call services/repos, build responses. Must always send ACK (see Error Handling below).
- **Services** (`svc_guild.go`, etc.): Business logic extracted from handlers. New domain logic should go here.
- **Repositories**: All SQL lives in `repo_*.go` files behind interfaces in `repo_interfaces.go`. The `Server` struct holds interface types, not concrete implementations. Handler code must never contain inline SQL.
- **Sign server** has its own repo pattern: 3 interfaces in `server/signserver/repo_interfaces.go`.

### Key Subsystems

| File(s) | Purpose |
|---------|---------|
| `sys_session.go` | Per-connection state: character, stage, semaphores, send queue |
| `sys_stage.go` | `StageMap` (`sync.Map`-backed), multiplayer rooms/lobbies |
| `sys_channel_server.go` | Server lifecycle, Raviente shared state, world management |
| `sys_semaphore.go` | Distributed locks for events (Raviente siege, guild ops) |
| `channel_registry.go` | Cross-channel operations (worldcast, session lookup, mail) |
| `handlers_cast_binary.go` | Binary state relay between clients (position, animation) |
| `handlers_helpers.go` | `loadCharacterData`/`saveCharacterData` shared helpers |
| `guild_model.go` | Guild data structures |

### Binary Serialization

`common/byteframe.ByteFrame` — sequential big-endian reads/writes with sticky error pattern (`bf.Err()`). Used for all packet parsing, response building, and save data manipulation. Use `encoding/binary` only for random-access reads at computed offsets on existing `[]byte` slices.

### Database

PostgreSQL with embedded auto-migrating schema in `server/migrations/`:
- `sql/0001_init.sql` — consolidated baseline
- `seed/*.sql` — demo data (applied via `migrations.ApplySeedData()` on fresh DB)
- New migrations: `sql/0002_description.sql`, etc. (each runs in its own transaction)

The server runs `migrations.Migrate()` automatically on startup.

### Configuration

Two reference files: `config.example.json` (minimal) and `config.reference.json` (all options). Loaded via Viper in `config/config.go`. All defaults registered in code. Supports 40 client versions (S1.0 → ZZ) via `ClientMode`. If `config.json` is missing, an interactive setup wizard launches at `http://localhost:8080`.

### Protocol Bot (`cmd/protbot/`)

Headless MHF client implementing the complete sign → entrance → channel flow. Shares `common/` and `network/crypto` but avoids `config` dependency via its own `conn/` package.

## Concurrency

Lock ordering: `Server.Mutex → Stage.RWMutex → semaphoreLock`. Stage map uses `sync.Map`; individual `Stage` structs have `sync.RWMutex`. Cross-channel operations go exclusively through `ChannelRegistry` — never access other servers' state directly.

## Error Handling in Handlers

The MHF client expects `MsgSysAck` for most requests. Missing ACKs cause client softlocks. On error paths, always send `doAckBufFail`/`doAckSimpleFail` before returning.

## Testing

- **Mock repos**: Handler tests use `repo_mocks_test.go` — no database needed
- **Table-driven tests**: Standard pattern (see `handlers_achievement_test.go`)
- **Race detection**: `go test -race` is mandatory in CI
- **Coverage floor**: CI enforces ≥50% total coverage

## Adding a New Packet

1. Define struct in `network/mhfpacket/msg_*.go` (implements `MHFPacket` interface: `Parse`, `Build`, `Opcode`)
2. Add packet ID constant in `network/packetid.go`
3. Register handler in `server/channelserver/handlers_table.go`
4. Implement handler in appropriate `handlers_*.go` file

## Adding a Database Query

1. Add method signature to the relevant interface in `repo_interfaces.go`
2. Implement in the corresponding `repo_*.go` file
3. Add mock implementation in `repo_mocks_test.go`

## Known Issues

See `docs/anti-patterns.md` for structural patterns and `docs/technical-debt.md` for specific fixable items with file paths and line numbers.

## Contributing

- Branch naming: `feature/`, `fix/`, `refactor/`, `docs/`
- Commit messages: conventional commits (`feat:`, `fix:`, `refactor:`, `docs:`)
- Update `CHANGELOG.md` under "Unreleased" for all changes
