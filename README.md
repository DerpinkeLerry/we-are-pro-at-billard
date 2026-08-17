# Pool Arena

Server-authoritatives Multiplayer-8-Ball für den Browser. PHP übernimmt Website, Identität, Lobby-Verwaltung und kurzlebige Join-Tickets; Go besitzt Lobby-, Match-, Regel- und Physics-State; PostgreSQL persistiert Konten, Lobbies, Matches und Statistiken; Three.js rendert den Tisch und interpoliert ausschließlich Serverzustand.

## Features

- öffentliche und private Lobbies mit Passwort, Invite-Code und 0/30/45/60-Sekunden Shot Timer
- Guest-Sessions sowie Registrierung/Login, Profil, Winrate und Match History
- sieben rein kosmetische Cue-Skins mit rotierender 3D-Vorschau
- drei Grafikprofile: Very High, Normal und Very Low; Very Low reduziert DPR, Geometrie, Schatten und Framerate auf 30 FPS
- server-authoritative 1v1-8-Ball-Logik mit Open Table, Solids/Stripes, Call Ball/Pocket, Safety, Fouls, Scratch, Ball-in-Hand und Break-Entscheidungen
- echte FIFO-Spielerrotation mit Spectators und Ready-Phase
- aktiver Reconnect-Slot mit Grace Period und Disconnect-Forfeit
- 120-Hz-Go-Physik mit `float64`, adaptiven Substeps, swept Ball-Ball CCD, Ball-/Cushion-/Jaw-Impulsen, Sliding/Rolling, Spin und Rest Detection
- geometrische Corner-/Side-Pockets mit Mouth, Jaw, Throat, Shelf, Back Draft und vertikalem Falling-State; keine magnetischen Pocket-Center
- binäre 30-Hz-Physics-Snapshots (`PLS1`) und zuverlässige JSON-Kontrollereignisse
- responsive Top-Down-Three.js-Ansicht, OrthographicCamera, Fullscreen, Spectator-Zoom, Touch, Maus und Pfeiltasten-Feinzielung
- Lobby-Chat mit Server-Timestamp, Limits, Escaping und clientseitigem Mute
- prozedurale WebAudio-SFX für Cue, Ball, Cushion, Pocket und Match-Ereignisse
- Development-Debug-Overlay für Collider, Pocket-Zonen, Ball-IDs, Velocity, Spin, FPS, Netzwerk und Renderer-Statistiken
- Docker Compose, GitHub Actions, Healthchecks und Render Blueprint

## Screenshots

Nach einem lokalen Start sind folgende Aufnahme-Slots vorgesehen, damit Screenshots aus der tatsächlich laufenden Build-Version stammen und nicht von der Implementierung abweichen:

| Ansicht | empfohlene Aufnahme |
|---|---|
| Landing / Lobby Browser | `/` und `/lobbies` |
| Join / Cue Preview | `/lobby/<CODE>` |
| Laufendes Match | `/play/<CODE>` mit zwei Spielern |
| Physics Debug | Development Settings → Debug Overlay |
| Very Low | laufendes Match mit Preset `Very Low` |

## Tech Stack

- PHP 8.4 + Apache, PDO und Vanilla-PHP-Router/Controller/Services
- Vanilla ES Modules + Three.js 0.185.1
- Go 1.23
- PostgreSQL 16
- Docker / Docker Compose
- Render Blueprint (`render.yaml`)
- GitHub Actions (`.github/workflows/ci.yml`)

Es gibt keinen Node.js-Runtime-Service.

## Architektur

```text
Browser
  | HTTPS                         | WSS
  v                               v
PHP Web Service              Go Game Service
  | Sessions / Lobby               | Lobby Actor
  | Join JWT                       | Queue / Reconnect
  | Profile / History              | Match / Rules / Physics
  +---------------+----------------+
                  |
             PostgreSQL
```

Wichtigste Vertrauensgrenze: Der Browser sendet niemals autoritative Kugelpositionen. Ein Stoß enthält nur Winkel, Power, Cue-Tip-Offset, Call-Daten, `requestId`, `matchId` und den aktuellen zufälligen `turnNonce`. Go prüft den Request und berechnet den gesamten Stoß.

Weitere Details:

- `docs/architecture/ARCHITECTURE.md`
- `docs/protocol/WEBSOCKET.md`
- `docs/physics/PHYSICS.md`
- `docs/security/SECURITY.md`
- `docs/testing/TESTING.md`
- `docs/deployment/RENDER.md`

## Repository

```text
.
├── config/                  # kanonische Table/Physics/Rules-Konfigurationen
├── database/migrations/     # PostgreSQL-Schema
├── docs/
├── game-server/
│   ├── cmd/pool-server/
│   ├── internal/
│   │   ├── app/ auth/ config/ lobby/ match/
│   │   ├── persistence/ physics/ protocol/ realtime/ rules/
│   └── tests/
├── web/
│   ├── public/assets/
│   ├── src/
│   └── templates/
├── compose.yaml
└── render.yaml
```

## Voraussetzungen

Für den einfachsten lokalen Start:

- Docker Engine mit Docker Compose v2
- Browser mit WebGL/WebAudio/WebSocket-Unterstützung
- Internetzugang des Browsers für das versionsfixierte Three.js-ES-Modul von jsDelivr

Für Entwicklung ohne Container zusätzlich:

- Go 1.23+
- PHP 8.4+ mit `pdo_pgsql` und `mbstring`
- PostgreSQL 16+

## Lokaler Start

```bash
git clone <dein-repository-url> pool-arena
cd pool-arena
docker compose up --build
```

Danach:

- Web: `http://localhost:8080`
- Go WebSocket: `ws://localhost:8081/ws`
- Go Ping: `http://localhost:8081/ping`
- PHP Health: `http://localhost:8080/health`
- Go Health: `http://localhost:8081/health`

Stoppen:

```bash
docker compose down
```

Daten inklusive PostgreSQL-Volume entfernen:

```bash
docker compose down -v
```

## Lokales A/B/C-Abnahmeszenario

1. Browser A öffnet `/lobbies`, erstellt eine Lobby und joint mit Cue/Grafikpreset.
2. A bleibt in der FIFO-Queue, solange kein zweiter Teilnehmer da ist.
3. Browser B joint. A und B werden reserviert, drücken `Ready`, Countdown startet, Match beginnt.
4. Browser C joint während des Matches als Spectator auf Queue-Position 1.
5. Nach Match-Ende werden A und B hinten an die bestehende Queue gehängt: C–A wird das nächste Paar, B wartet.
6. Danach folgt B–C, sofern alle verbunden bleiben.
7. Für Reconnect die Netzwerkverbindung eines aktiven Browsers kurz trennen; sein Slot bleibt während der konfigurierten Grace Period reserviert und der Match-State pausiert.

## Environment

`.env.example` dokumentiert die Variablen. Compose setzt sichere Entwicklungswerte direkt. Produktion muss eigene zufällige Secrets verwenden.

| Variable | Zweck |
|---|---|
| `APP_ENV` | `development` / `production` |
| `APP_BASE_URL` | öffentliche PHP-Origin |
| `GAME_PUBLIC_WS_URL` | Browser-URL des Game-WebSockets |
| `GAME_INTERNAL_URL` | PHP → Go URL für Runtime-Lobbydaten |
| `DATABASE_URL` | PostgreSQL DSN |
| `JOIN_TOKEN_SECRET` | HMAC-Secret für Join-JWT, mindestens 32 Bytes |
| `GAME_INTERNAL_SECRET` | Secret für internen Lobby-Status, mindestens 32 Bytes |
| `ALLOWED_ORIGINS` | kommaseparierte erlaubte Browser-Origins für WSS/Ping |
| `PORT` | Go HTTP-Port |

## Datenbank

`database/migrations/001_init.sql` erzeugt:

- `users`, `guest_sessions`, `auth_sessions`
- `lobbies`
- `matches`, `match_players`, `shots`, `match_checkpoints`
- `player_statistics`

Der Go-Server schreibt keine 120-Hz-Ticks in PostgreSQL. Persistiert werden Match-Grenzen, akzeptierte Shots, Ergebnisse, Statistiken und kompakte Checkpoints.

Die Web-Container-Startsequenz führt `php /var/www/html/bin/migrate.php` idempotent aus.

## PHP Service

Wesentliche Routen:

- `GET /health`
- `GET /api/session`
- `POST /api/guest`
- `POST /api/register`
- `POST /api/login`
- `POST /api/logout`
- `GET|POST /api/lobbies`
- `POST /api/lobbies/{CODE}/ticket`
- `GET /api/profile`
- `GET /api/matches`

Schreibende API-Requests verwenden den Session-CSRF-Token. Lobby-Passwörter werden nur in PHP validiert und niemals an den Go-Server weitergereicht.

## Go Service

Öffentlich:

- `GET /health`
- `GET /ping`
- `GET /ws` (WebSocket Upgrade; erste Nachricht muss `AUTH` sein)

Intern:

- `GET /internal/lobbies` mit `X-Internal-Secret`

Jede aktive Lobby läuft als einzelner Actor/Goroutine mit exklusivem Besitz ihres Runtime-States. V1 nutzt genau eine autoritative Go-Instanz; horizontales Skalieren erfordert eine explizite Lobby-Ownership-/Actor-Verteilung und ist nicht aktiviert.

## Development

Alle Go-Tests:

```bash
cd game-server
go test ./...
go test -race ./...
go vet ./...
```

WebSocket-Integrationstests:

```bash
cd game-server
go test -tags=integration ./tests/network
```

PHP-Syntax:

```bash
find web -name '*.php' -print0 | xargs -0 -n1 php -l
```

JavaScript-Syntax, wenn Node nur als lokales Entwickler-Lintwerkzeug vorhanden ist:

```bash
find web/public/assets/js -name '*.js' -print0 | xargs -0 -n1 node --check
```

Node wird dafür nicht als Anwendungslaufzeit verwendet.

## Production Build

```bash
docker compose build --pull
```

Das Go-Dockerfile erzeugt ein statisches Binary in einem Multi-Stage-Build. Das PHP-Image installiert nur die benötigten PHP-Erweiterungen und kopiert die gemeinsame Table-Konfiguration unter `/public/config`.

## Render.com

`render.yaml` definiert:

- `pool-web` als Docker Web Service
- `pool-game` als Docker Web Service
- `pool-db` als PostgreSQL-Datenbank
- Healthchecks und Secrets/URLs als Environment-Einträge

Setze `pool-game` für V1 auf genau **eine Instanz**. Trage danach im Render-Dashboard die öffentlichen und internen URLs sowie starke, auf beiden Services identische Secrets ein. Details stehen in `docs/deployment/RENDER.md`.

## WebSocket-Konfiguration

PHP stellt ein 60 Sekunden gültiges, signiertes Join-Ticket aus. Die erste WebSocket-Nachricht lautet:

```json
{"type":"AUTH","token":"<short-lived-jwt>"}
```

Danach werden Kontrollereignisse als JSON übertragen. Bewegungsframes sind binäre `PLS1`-Snapshots mit Sequenznummer und Serverzeit. Der Client verwirft veraltete Snapshots und interpoliert für die Anzeige.

## Troubleshooting

**`server_misconfigured` beim Web-Service**  
`JOIN_TOKEN_SECRET` oder `GAME_INTERNAL_SECRET` ist kürzer als 32 Bytes oder fehlt.

**WebSocket verbindet nicht**  
`GAME_PUBLIC_WS_URL` und `ALLOWED_ORIGINS` prüfen. In Produktion muss die Browser-Verbindung `wss://` verwenden.

**Lobbies werden im Browser ohne Runtime-Spieler angezeigt**  
`GAME_INTERNAL_URL` und `GAME_INTERNAL_SECRET` zwischen PHP und Go prüfen.

**Three.js lädt nicht**  
Der Browser muss das fest gepinnte ES-Modul von `cdn.jsdelivr.net` laden dürfen. CSP und Netzwerkzugriff prüfen.

**Datenbank noch nicht bereit**  
Compose wartet über `pg_isready`; bei manueller Entwicklung zuerst PostgreSQL starten und danach `php web/bin/migrate.php` ausführen.

**Debug-Collider fehlen**  
Nur in `APP_ENV=development`: Settings → Developer Debug Overlay aktivieren.

## Regel- und Geometriequellen

Die Projektkonfiguration referenziert die World Pool-Billiard Association (WPA):

- Rules of Play 2026: `https://www.wpapool.com/wp-content/uploads/2026/01/2026.01.02-WPA-Rules.pdf`
- Recommended Equipment Specifications: `https://wpapool.com/wp-content/uploads/2024/01/RECOMMENDED-EQUIPMENT-SPECIFICATIONS.pdf`

Projektwerte innerhalb offizieller Spannen sind in `config/table/wpa-9ft-v1.json` zentral festgeschrieben; Renderer und Physics lesen dieselbe Quelle.
