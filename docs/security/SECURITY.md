# Security

## Browser als nicht vertrauenswürdig

Der Client sendet keine autoritativen Kugelpositionen. Go validiert Match, Seat, State, `turnNonce`, Power, Spin, Ball-in-Hand und Call-Daten serverseitig.

## Sessions

- opaque Session-Cookie
- in SQLite liegt nur der SHA-256-Hash des Cookie-Tokens
- `HttpOnly`
- in Production `Secure`
- `SameSite=Lax`
- CSRF-Token für schreibende HTTP-Aktionen

## Join-Tickets

PHP erzeugt kurzlebige HMAC-JWTs mit unter anderem:

- `iss=pool-web`
- `aud=pool-game`
- Lobby-ID/-Code
- Participant
- `jti`
- Ablaufzeit

Go akzeptiert das Ticket nur als erste `AUTH`-Nachricht der WebSocket-Verbindung.

## Interne PHP↔Go-Kommunikation

`GAME_INTERNAL_SECRET` schützt interne Lobby- und Persistenzendpunkte. Im Single-Service-Container wird das Secret automatisch erzeugt und an beide Prozesse vererbt.

## WebSocket Origin

Go akzeptiert:

- explizit konfigurierte Origins
- oder dieselbe Origin/Host-Kombination wie der öffentliche Apache-Request

Apache verwendet `ProxyPreserveHost`, damit die Same-Origin-Prüfung auch hinter dem Reverse Proxy funktioniert.

## Chat

Chat bleibt Plain Text. Browsercode rendert Nutzerinhalte escaped und nicht als vertrauenswürdiges HTML.
