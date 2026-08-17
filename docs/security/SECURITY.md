# Security-Modell

## Grundsatz

Der Browser ist nicht vertrauenswürdig. Er darf Eingabeabsichten senden, aber niemals Ball-State, Fouls, Turnwechsel, Gruppen oder Gewinner festlegen.

## PHP-Sessions

- opaque Session-Cookies; in PostgreSQL wird nur ein SHA-256-Hash des Tokens gespeichert
- `HttpOnly`, `SameSite=Lax`, in Produktion `Secure`
- eigener CSRF-Token pro Session für schreibende HTTP-Requests
- `password_hash()` / `password_verify()` für Account- und Lobby-Passwörter
- PDO Prepared Statements
- Nickname-, Lobby-, Timer- und Cue-Allowlist-Validierung

## Join-JWT

PHP erstellt nach erfolgreicher Lobbyprüfung ein HS256-JWT mit maximal 60 Sekunden Lebensdauer. Claims enthalten u. a.:

- `iss=pool-web`, `aud=pool-game`
- `sub`, Principal-Typ/-ID
- Nickname und kosmetischen Cue-Skin
- Lobby-ID/-Code/-Name
- Shot-Timer und Config-Versionen
- `iat`, `nbf`, `exp`, zufälliges `jti`

Go prüft Algorithmus, Signatur, Issuer, Audience, Zeitgrenzen und notwendige Claims. Verwendete `jti` werden bis zum Ablauf im Replay-Cache blockiert.

`JOIN_TOKEN_SECRET` und `GAME_INTERNAL_SECRET` müssen mindestens 32 Bytes lang sein; der Prozess startet sonst nicht.

## WebSocket

- Produktion nur über TLS/WSS
- strikte `Origin`-Allowlist
- maximal 5 Sekunden bis zur ersten `AUTH`-Nachricht
- 16 KiB Client-Frame-Limit
- Read/Pong-Deadlines
- Connection-Limit pro IP
- globale Nachrichtenrate pro Participant
- separates Chat- und Shot-Rate-Limit
- unbekannte/zu lange Message Types werden ignoriert
- Spectators werden serverseitig vor Shot-Verarbeitung abgewiesen

## Gameplay-Replay-Schutz

- zufälliger `turnNonce` für jeden neuen Turn
- `requestId`-Deduplizierung
- Match-ID-Validierung
- Active-Seat-/Turn-Prüfung
- Power und Cue-Tip-Offset werden auf endliche, erlaubte Bereiche geprüft
- Ball-in-Hand-Position wird gegen Tisch, Pocket-Mouths und Ball-Overlaps validiert

## Reconnect

Reconnect verwendet ein frisches signiertes Join-Ticket. Der signierte Principal ist der stabile Schlüssel. Trifft eine neue Connection für denselben Principal ein, schließt Go die alte Connection und bindet den vorhandenen Participant um. Dadurch gibt es keine doppelte Spielerinstanz.

## Chat / XSS

- Server trimmt, entfernt unzulässige Control Characters und begrenzt auf 300 Runen
- Rate Limit 5 Nachrichten / 10 Sekunden
- UI fügt Nickname/Text/Timestamp über DOM `textContent`/Textnodes ein
- clientseitiges Mute wird in Local Storage gehalten

## Browser-Header

PHP setzt:

- Content-Security-Policy
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- restriktive `Permissions-Policy`
- `frame-ancestors 'none'`

## Interner Go-Endpunkt

`/internal/lobbies` verlangt `X-Internal-Secret` und vergleicht das Secret constant-time. Die Antwort enthält nur öffentliche Runtime-Zusammenfassungen, keine Tokens oder Passwörter.

## Logging

Secrets, Join-Tokens und Session-Cookies werden nicht geloggt. PHP schreibt strukturierte JSON-Ereignisse für Lobby-Erstellung und Fehler; Go nutzt `slog`-JSON für Join, Disconnect/Reconnect, Shot-Rejections, Match-Start/-Ergebnis, Persistenzfehler und Shutdown.
