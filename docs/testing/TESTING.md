# Teststrategie

## Go Unit-/Regressionstests

```bash
cd game-server
go test ./...
go test -race ./...
go vet ./...
```

Abgedeckt sind unter anderem:

### Physics

- Ball-Ball frontal, schräg, ruhender Ball, zwei bewegte Bälle
- High-Speed swept Collision Detection
- Cushion senkrecht, Winkel, Energieverlust
- Corner-/Side-Pocket unterschiedliche Geometrie
- zwölf Jaw-Segmente / sechs Pockets
- zentrierter Throat-Eintritt ohne Center-Attraction
- High-Speed Jaw Grazing ohne Circle-Capture
- Sliding-Slip-Abbau, Rolling/Rest
- Topspin vs Draw
- Side Spin und Cushion-Pfad

### Rules

- Break/Open Table
- Scratch und Ball-in-Hand
- falscher First Contact
- keine Rail nach Kontakt
- Safety
- 8-Ball legal/illegal
- Open-Table 8 als First Contact
- Break-Optionen

### Lobby/Auth

- JWT-Tampering, Ablauf und JTI-Replay
- Power/Spin/TurnNonce-Validierung
- Ball-in-Hand-Placement
- exakte A/B/C-FIFO-Rotation
- einzelner Teilnehmer bleibt bis zum zweiten in der Queue
- Spectator wird beim Disconnect aus Queue entfernt

## WebSocket-Integration

```bash
cd game-server
go test -tags=integration ./tests/network
```

Szenarien:

- ungültiger Token
- zwei Active Players plus queued Spectator
- Spectator-Shot wird abgewiesen
- Reconnect behält denselben Participant
- Out-of-turn-Shot
- Shot Spam Rate Limit
- Disconnect-Forfeit und anschließende Rotation
- leere Runtime-Lobby schließt

## Web-Syntax

```bash
find web -name '*.php' -print0 | xargs -0 -n1 php -l
find web/public/assets/js -name '*.js' -print0 | xargs -0 -n1 node --check
```

Der zweite Befehl verwendet Node ausschließlich als optionales Syntax-Lintwerkzeug und nicht als Runtime des Produkts.

## Manuelle Browser-Abnahme

- Very High / Normal / Very Low im selben Matchzustand vergleichen
- Very Low: 30-FPS-Cap, DPR 1, keine Schatten, geringe Segmente im Debug-Overlay prüfen
- Resize und Fullscreen in mehreren Seitenverhältnissen
- Touch-Zielen/Power/Spin auf Mobilgerät
- Spectator-Zoom verändert keinen Serverzustand
- Debug-Collider visuell deckungsgleich mit Rail/Jaw/Mouth/Throat
- Chat Timestamp, Spam-Limit und Mute
- Ping/Connection-State sichtbar
- Browser-DevTools-Manipulation von JS darf keine Ballposition oder fremden Turn autoritativ ändern

## Low-End-Metriken

Development Settings → Debug Overlay zeigt:

- FPS / Frame Time
- Physics 120 Hz / Server Tick
- Snapshot Rate / Ping
- Match State / Shot Number
- Cue-Speed / Angular Speed
- Draw Calls / Triangles / Geometries / Textures
- DPR / Grafikpreset / Renderer
