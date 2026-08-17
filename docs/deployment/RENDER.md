# Render Deployment

## Ressourcen

Der Root-Blueprint `render.yaml` definiert:

- `pool-web`: Docker Web Service, PHP/Apache
- `pool-game`: Docker Web Service, Go
- `pool-db`: PostgreSQL

Beide Services erhalten einen HTTP-Healthcheck auf `/health`.

## V1-Instanzmodell

`pool-game` muss auf genau einer autoritativen Instanz laufen. Der Runtime-Lobby-State ist absichtlich in-memory und Actor-basiert; mehrere unabhängige Game-Instanzen ohne Lobby-Routing würden unterschiedliche Wahrheiten besitzen.

## Environment

Auf `pool-web`:

```text
APP_ENV=production
APP_BASE_URL=https://<web-domain>
GAME_PUBLIC_WS_URL=wss://<game-domain>/ws
GAME_INTERNAL_URL=<von PHP erreichbare Go-Service-URL>
DATABASE_URL=<Render PostgreSQL>
JOIN_TOKEN_SECRET=<starkes gemeinsames Secret>
GAME_INTERNAL_SECRET=<starkes gemeinsames Secret>
```

Auf `pool-game`:

```text
APP_ENV=production
DATABASE_URL=<Render PostgreSQL>
JOIN_TOKEN_SECRET=<gleich wie web>
GAME_INTERNAL_SECRET=<gleich wie web>
ALLOWED_ORIGINS=https://<web-domain>
PORT=<von Render bereitgestellt>
```

Secrets dürfen nicht in Git oder `render.yaml` festgeschrieben werden.

## Datenbank

Das PHP-Dockerimage führt den idempotenten Migration Runner beim Containerstart aus. Für kontrollierte Releases kann derselbe Befehl als Pre-Deploy-/One-Off-Schritt ausgeführt werden:

```bash
php /var/www/html/bin/migrate.php
```

## Healthchecks

`pool-web /health` prüft eine DB-Abfrage und gibt nur Service-Status zurück.

`pool-game /health` gibt aus:

```json
{
  "status":"ok",
  "database":"ok",
  "activeLobbies":0,
  "connections":0
}
```

Ein kurzzeitiger DB-Fehler wird im Go-Health-Body als `degraded` ausgewiesen, ohne Tokens oder personenbezogene Daten zu veröffentlichen.

## Graceful Shutdown

Beim SIGTERM:

1. HTTP-Server nimmt keine neuen Verbindungen mehr an.
2. Lobby-Actors erhalten einen Shutdown-Befehl.
3. Ein nicht finales Match schreibt einen stabilen Checkpoint.
4. Clients erhalten `SERVER_ERROR: server_restarting` und die WebSockets werden geschlossen.
5. Browser versuchen automatisch neu zu verbinden.

Der Checkpoint ist für Diagnose/Recovery-Bausteine persistiert; V1 stellt einen durch Prozess-Restart unterbrochenen Match-State nicht automatisch aus PostgreSQL wieder her. Ein normaler Client-Network-Reconnect innerhalb eines laufenden Game-Prozesses wird vollständig unterstützt.

## Domains/TLS

Die Browserseite muss die Go-Origin über `wss://` erreichen. `ALLOWED_ORIGINS` enthält die exakte PHP-Origin, nicht die WebSocket-Origin. `/ping` erlaubt CORS nur für dieselbe Allowlist, damit der Lobby-Browser die echte Browser→Gateway-Latenz messen kann.

## Post-Deploy-Smoke-Test

1. `/health` beider Services aufrufen.
2. öffentliche Lobby anlegen und mit zwei getrennten Browser-Sessions joinen.
3. Ready/Countdown und einen Shot testen.
4. dritten Spectator joinen; Queue-Position kontrollieren.
5. aktive Verbindung kurz offline/online setzen und Reconnect prüfen.
6. Match History nach einem abgeschlossenen/forfeiteten Match prüfen.
