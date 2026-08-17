# Architektur

## Komponenten

### PHP Control Plane

- Guest-/Account-Sessions und CSRF
- Lobby-Browser, Lobby-Erstellung, private Passwörter
- Profil, Statistik und Match History
- Validierung von Nickname/Cue-Skin
- kurzlebige HMAC-signierte Join-JWTs
- Lesen der Go-Runtime-Lobbyübersicht über geschützten internen HTTP-Endpunkt

### Go Gameplay Plane

- WebSocket-Authentifizierung und Rate Limits
- ein Actor/Goroutine pro Lobby
- Active Seats, Spectator-FIFO, Ready, Countdown, Shot Timer
- Reconnect/Forfeit
- Match State Machine und vollständige serverseitige Shot-Validierung
- Physics Engine und getrennte 8-Ball Rules Engine
- Match-/Shot-/Checkpoint-Persistenz

### Browser / Three.js

- Website-Navigation und UI
- Eingaben als Requests, niemals als autoritativer State
- Orthographic Top-Down Rendering
- Snapshot-Interpolation und harte Korrektur bei finalen Keyframes
- Grafikpresets, Audio und Debug-Visualisierung

### PostgreSQL

- Identitäten und Web-Sessions
- Lobby-Metadaten
- Matches und Match-Player-Snapshots
- akzeptierte Shots und Final-State-Hashes
- aggregierte Statistiken
- stabile Match-Checkpoints

## Datenfluss beim Join

```text
Browser -> PHP POST /api/lobbies/{code}/ticket
PHP     -> prüft Session, CSRF, Lobby-Passwort, Cue, Nickname
PHP     -> signiert JWT (60s, jti, lobby/principal/cue)
Browser -> WSS /ws, erste Nachricht AUTH
Go      -> prüft Origin, Signatur, Claims, Ablauf, JTI
Go      -> bindet/ersetzt Participant-Connection
Go      -> AUTH_OK + LOBBY_STATE + optional MATCH_KEYFRAME
```

Lobby-Passwörter verlassen PHP nicht.

## Shot-Datenfluss

```text
Client Input
   |
   v
SHOT_REQUEST
   |
   v
Go Validation
   |
   v
120-Hz Physics + adaptive Substeps
   |
   +-> ShotReport
   |      |
   |      v
   |   Rules Engine
   |      |
   +------+
      Match Outcome
         |
         +-> JSON Events
         +-> 30-Hz binary Playback Snapshots
         +-> PostgreSQL Shot/Checkpoint
```

Die Simulation wird für einen akzeptierten Stoß vollständig vorausberechnet. Das ist bei Pool möglich, weil während rollender Kugeln kein neuer legitimer Shot-Input akzeptiert wird.

## Lobby-State-Machine

```text
WAITING -> STARTING -> PLAYING -> POST_GAME -> ROTATING
   ^          |                                  |
   +----------+----------------------------------+
   |
   +--------------------------------------> CLOSING (leer)
```

Ein einzelner Teilnehmer bleibt in `WAITING` in der FIFO-Queue. Erst wenn zwei verbundene Teilnehmer verfügbar sind, werden beide Active Seats reserviert und die Ready-Phase startet.

## Match-State-Machine

```text
TURN_AWAITING_SHOT
   | shot accepted
   v
BALLS_MOVING
   | resolve
   +-> TURN_AWAITING_SHOT
   +-> BALL_IN_HAND
   +-> BREAK_OPTION
   +-> MATCH_FINISHED

aktive Disconnects: jeder nicht-finale State -> PAUSED -> vorheriger State
```

## FIFO-Rotation

Active Seats sind nicht gleichzeitig Queue-Einträge. Nach Match-Ende werden die weiterhin verbundenen bisherigen Active Players hinten an die bestehende Queue gehängt.

```text
vor Match 1: Active A,B | Queue C
nach Match 1: Queue C,A,B -> Active C,A | Queue B
nach Match 2: Queue B,C,A -> Active B,C | Queue A
```

Gewinner und Verlierer haben keine Priorität.

## Skalierung

V1 hält den Lobby-State im Go-Prozess und betreibt deshalb genau eine autoritative Game-Service-Instanz. Eine spätere horizontale Skalierung benötigt explizite Lobby-Ownership, Routing oder einen verteilten Actor-/State-Layer; einfach mehrere identische In-Memory-Instanzen zu starten wäre nicht korrekt.
