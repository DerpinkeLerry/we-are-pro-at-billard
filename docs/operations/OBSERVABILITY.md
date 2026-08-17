# Operations und Observability

## Strukturierte Logs

Go schreibt JSON über `log/slog`. Relevante Events sind Serverstart/-shutdown, WebSocket-Authfehler, Join, Reconnect, Disconnect, Shot-Rejection, Matchstart/-ergebnis, Checkpoint-/Persistenzfehler und das Schließen leerer Lobbies.

PHP verwendet JSON-Zeilen über `error_log()` für Lobby-Erstellung sowie Datenbank-/Runtimefehler.

## Datenschutz

Nicht loggen:

- Session-Cookies
- Join-JWTs
- Lobby-Passwörter
- `JOIN_TOKEN_SECRET`
- `GAME_INTERNAL_SECRET`

Participant Public IDs und Lobby-Codes dürfen für technische Korrelation verwendet werden; persistente Secrets nicht.

## Health

- PHP: `/health`
- Go: `/health`
- Go Browser-Latency Probe: `/ping`

Health-Antworten enthalten keine Nicknames, Tokens oder Matchinhalte.
