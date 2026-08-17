# WebSocket-Protokoll v1

## Verbindung

Endpoint: `/ws`.

Die erste Clientnachricht muss innerhalb von fünf Sekunden `AUTH` sein. Vor erfolgreicher Authentifizierung werden keine Gameplay-Nachrichten verarbeitet. Produktion verwendet `wss://` und eine explizite Origin-Allowlist.

```json
{"type":"AUTH","token":"<short-lived-join-jwt>"}
```

Kontrollnachrichten sind JSON. Laufende Physics-Snapshots sind binär. Jede Servernachricht besitzt intern eine monotone Sequenznummer; der Browser verwirft ältere Frames.

## Client → Server

- `AUTH`
- `READY_SET`
- `SHOT_REQUEST`
- `BALL_IN_HAND_PLACE`
- `CHAT_SEND`
- `CLIENT_PING`
- `BREAK_OPTION`
- `LEAVE`

Ein `SHOT_REQUEST` enthält:

```json
{
  "type":"SHOT_REQUEST",
  "requestId":"uuid",
  "matchId":"uuid",
  "turnNonce":"random-turn-nonce",
  "aimAngle":0.0,
  "power":0.55,
  "cueOffsetX":0.0,
  "cueOffsetY":0.0,
  "calledBall":3,
  "calledPocket":0,
  "safety":false
}
```

Der Server akzeptiert niemals Ballpositionen, Geschwindigkeiten oder Regelresultate vom Client.

## Server → Client JSON

Tatsächlich emittierte Eventtypen:

- `AUTH_OK`, `AUTH_FAILED`
- `LOBBY_STATE`, `READY_STATE`, `COUNTDOWN`
- `MATCH_STARTED`, `MATCH_KEYFRAME`
- `TURN_STARTED`
- `SHOT_ACCEPTED`, `SHOT_REJECTED`, `COLLISION_EVENTS`, `SHOT_RESOLVED`
- `FOUL`, `BREAK_OPTION_REQUIRED`
- `PLAYER_RECONNECTING`, `PLAYER_RECONNECTED`
- `MATCH_FINISHED`, `NEXT_MATCH`
- `CHAT_MESSAGE`
- `PONG`, `SERVER_ERROR`

`LOBBY_STATE` enthält Teilnehmerrollen, Queue, Active Seats, Ready-/Reconnect-Status, Shot-Deadline und – falls vorhanden – den aktuellen Matchzustand.

## Binäres Physics-Format `PLS1`

Little Endian:

```text
4 bytes   magic "PLS1"
8 bytes   uint64 sequence
8 bytes   int64 serverTime unix ms
4 bytes   float32 simulationTime seconds
1 byte    matchId UTF-8 length
N bytes   matchId
1 byte    ball count
per ball:
  1 byte  ball id
  1 byte  state enum: table/falling/pocketed/off_table
  1 byte  int8 pocket id (-1 if none)
  8 x 4   float32: x,y,z,vx,vy,wx,wy,wz
```

Während Bewegung erzeugt die 120-Hz-Simulation ungefähr 30 Netzframes pro Simulationssekunde. Pro Connection existiert ein coalescing Snapshot-Slot: Wenn ein Client langsam ist, wird ein alter Physics-Frame durch den neuesten ersetzt, statt eine große Queue aufzubauen. JSON-Kontrollereignisse liegen auf einer separaten zuverlässigen Queue.

## Replay- und Turn-Schutz

- Join-JWTs besitzen `jti` und kurze Ablaufzeit; verwendete `jti` werden bis zum Ablauf blockiert.
- Jeder Turn erhält einen neuen zufälligen `turnNonce`.
- `requestId` wird pro Participant zeitlich dedupliziert.
- `matchId`, Active Seat, Match-State, Shot Timer, Power, Cue-Offset, Call-Daten und Spectator-Rolle werden serverseitig validiert.

## Reconnect

Der Browser holt für jeden Wiederverbindungsversuch ein frisches PHP-Join-Ticket. Go identifiziert den bestehenden Participant über den signierten Principal und ersetzt die alte Connection; es wird kein zweiter Spieler erzeugt. Ein aktiver Slot bleibt bis zur serverseitigen Reconnect-Deadline reserviert und pausiert ein laufendes Match.
