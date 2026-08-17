# Pool Arena — Single Service Edition

Server-authoritatives Multiplayer-8-Ball für den Browser. Diese Edition ist absichtlich so gebaut, dass auf Render **nur ein einziger Docker Web Service** benötigt wird.

Im selben Container laufen:

- Apache + PHP für Website, Sessions, Accounts, Lobbies und Join-Tickets
- Go für WebSocket, Rotation, Match-State, Regeln und 120-Hz-Physics
- SQLite für Sessions, Lobbies, Match-History und Statistiken
- Three.js im Browser für Rendering und Interpolation

Apache ist der einzige öffentliche Listener. `/ws` und `/ping` werden intern an den Go-Prozess weitergeleitet. PHP und Go teilen automatisch generierte Secrets; auf Render müssen dafür keine Environment-Variablen eingerichtet werden.

## Features

- öffentliche/private Lobbies, Passwort, Invite-Code und Shot Timer
- Guests sowie Registrierung/Login, Profil, Statistiken und Match-History
- sieben kosmetische Cue-Skins
- Very High / Normal / Very Low Grafikprofile
- server-authoritatives 8-Ball mit Call Ball/Pocket, Safety, Fouls, Scratch und Ball-in-Hand
- FIFO-Rotation, Spectators, Ready-System und Reconnect-Grace-Period
- 120-Hz-Go-Physics mit CCD, Spin, Reibung, Cushion/Jaw-Kollisionen und geometrischen Pockets
- binäre Physics-Snapshots plus JSON-Control-Events
- Maus, Tastatur und Touch
- Lobby-Chat, Audio, Fullscreen und Debug-Overlay

## Architektur

```text
Browser
   |
   | HTTPS / WSS
   v
+------------------------------------------+
| EIN Render Web Service / EIN Container   |
|                                          |
| Apache :$PORT                            |
|   |                                      |
|   +--> PHP App                           |
|   |      +--> SQLite                     |
|   |                                      |
|   +--> /ws,/ping --> Go :8081            |
|                       |                  |
|                       +--> PHP internal  |
|                            persistence   |
+------------------------------------------+
```

Der Browser ist niemals autoritativ. Er sendet nur Eingaben wie Zielwinkel, Power, Cue-Offset, Call-Daten und `turnNonce`. Go entscheidet über gültige Aktionen, berechnet die komplette Physik und wertet die Regeln aus.

## Repository

```text
.
├── Dockerfile                    # EIN Produktions-Container
├── compose.yaml                  # lokaler Ein-Service-Start
├── render.yaml                   # optional; ebenfalls nur ein Service
├── config/                       # gemeinsame Tisch/Physics/Rules-Konfiguration
├── database/migrations/          # SQLite-Schema
├── docker/single-service/        # Apache-Proxy + Entrypoint
├── game-server/                  # Go Realtime/Rules/Physics
├── web/                          # PHP + Three.js Client
└── docs/
```

## Lokaler Start

Voraussetzung: Docker + Docker Compose.

```bash
docker compose up --build
```

Danach:

- App: `http://localhost:8080`
- Health: `http://localhost:8080/health`
- Game Ping: `http://localhost:8080/ping`
- WebSocket: `ws://localhost:8080/ws`

Stoppen:

```bash
docker compose down
```

## Render: nur ein Web Service

1. GitHub-Repo öffnen und sicherstellen, dass `Dockerfile` direkt im Repo-Root liegt.
2. Render → **New → Web Service**.
3. GitHub-Repo auswählen.
4. Branch `main`.
5. Runtime/Language: **Docker**.
6. Instance Type: **Free** (wenn du kostenlos testen willst).
7. Root Directory leer lassen.
8. Es sind **keine Environment Variables, keine Datenbank und kein zweiter Service erforderlich**.
9. Create Web Service.

Render baut automatisch den Root-`Dockerfile`. Der Container bindet Apache an das von Render gesetzte `$PORT`; Go läuft ausschließlich intern auf Port `8081`.

### Wichtige Free-Tier-Einschränkung

Die Single-Service-Edition nutzt SQLite im lokalen Container-Dateisystem. Auf einem kostenlosen Render Web Service ist dieses Dateisystem nicht persistent. Nach Spin-down, Restart oder Redeploy können daher Accounts, Lobbies, Statistiken und Match-History zurückgesetzt werden.

Das laufende Multiplayer-Spiel selbst benötigt trotzdem keinen zusätzlichen Service. Wenn später dauerhafte Daten gewünscht sind, kann die Persistenz wieder auf eine externe Datenbank umgestellt werden.

## Automatische Konfiguration

Beim Containerstart passiert automatisch:

1. SQLite-Datei und Verzeichnis anlegen.
2. Datenbankmigrationen ausführen.
3. `JOIN_TOKEN_SECRET` generieren, falls nicht gesetzt.
4. `GAME_INTERNAL_SECRET` generieren, falls nicht gesetzt.
5. Apache auf `$PORT` konfigurieren.
6. Apache starten.
7. Go-Game-Server intern auf `127.0.0.1:8081` starten.
8. `/ws` und `/ping` von Apache zu Go proxien.

Du musst auf Render also keine URLs oder Secrets zwischen Diensten kopieren.

## Optionale Environment-Overrides

Keine Variable ist für den normalen Render-Start erforderlich.

| Variable | Standard | Zweck |
|---|---|---|
| `PORT` | von Render / lokal 10000 | öffentlicher Apache-Port |
| `APP_ENV` | `production` | Runtime-Modus |
| `SQLITE_PATH` | `/var/lib/pool/pool.sqlite` | SQLite-Datei |
| `JOIN_TOKEN_SECRET` | beim Boot generiert | Join-JWT HMAC |
| `GAME_INTERNAL_SECRET` | beim Boot generiert | interne PHP↔Go-Authentifizierung |
| `GAME_PUBLIC_WS_URL` | automatisch same-origin | optionaler WS-Override |
| `ALLOWED_ORIGINS` | same-origin wird automatisch akzeptiert | zusätzliche Origin-Allowlist |

## Datenbank

`database/migrations/001_init.sql` erzeugt:

- `users`
- `guest_sessions`
- `auth_sessions`
- `lobbies`
- `matches`
- `match_players`
- `shots`
- `player_statistics`
- `match_checkpoints`

Go persistiert Match-Grenzen über abgesicherte interne HTTP-Endpunkte der PHP-App. 120-Hz-Physics-Ticks werden nicht in SQLite geschrieben.

## HTTP- und WebSocket-Routen

Öffentlich:

- `GET /health`
- `GET /ping`
- `GET /ws` — WebSocket Upgrade
- `GET /api/session`
- `POST /api/guest`
- `POST /api/register`
- `POST /api/login`
- `POST /api/logout`
- `GET|POST /api/lobbies`
- `POST /api/lobbies/{CODE}/ticket`
- `GET /api/profile`
- `GET /api/matches`

Interne Persistenzrouten sind durch `X-Internal-Secret` geschützt und werden nur vom Go-Prozess verwendet.

## Tests

Go-Kernpakete:

```bash
cd game-server
go test ./internal/physics ./internal/rules ./internal/match ./internal/persistence
```

Vollständige Suite mit externem Modulzugriff:

```bash
cd game-server
go test ./...
go test -race ./...
go vet ./...
```

PHP:

```bash
find web -name '*.php' -print0 | xargs -0 -n1 php -l
```

JavaScript:

```bash
find web/public/assets/js -name '*.js' -print0 | xargs -0 -n1 node --check
```

## Weitere Dokumentation

- `docs/architecture/ARCHITECTURE.md`
- `docs/deployment/RENDER.md`
- `docs/protocol/WEBSOCKET.md`
- `docs/physics/PHYSICS.md`
- `docs/security/SECURITY.md`
- `docs/testing/TESTING.md`

## Regel- und Geometriequellen

Die gemeinsame Projektkonfiguration basiert auf den dokumentierten WPA-Regeln und Equipment-Spezifikationen. Die konkreten Projektwerte liegen versioniert in `config/` und werden von Renderer und Physics aus derselben Quelle gelesen.
