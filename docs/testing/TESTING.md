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
- stationärer Extrem-Side-Spin gibt den nächsten Turn innerhalb von 2,2 Sekunden frei

### Rules

- Red/Yellow-Zuordnung auf der offenen Tabelle
- legaler Break ohne Pot übergibt den offenen Tisch an den Gegner
- Break-Pot lässt die Farben zunächst offen
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
- Solo-Training besitzt beide Sitze mit einer Verbindung
- ein echter zweiter Teilnehmer ersetzt vor Matchbeginn den virtuellen Solo-Sitz
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
- Gegner erhält validierte Live-Queue-/Aim-Updates
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
- Maus/Touch: zielen, gedrückt halten, Ziellinie fixieren, nach unten ziehen und loslassen
- horizontaler Kraftbalken, Kontaktpunkt und Gegner-Cue während der Bewegung
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
