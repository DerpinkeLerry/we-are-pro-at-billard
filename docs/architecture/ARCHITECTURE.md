# Architektur

## Ziel

Ein server-authoritatives Multiplayer-8-Ball-System, das auf Render als **ein einzelner Docker Web Service** betrieben werden kann.

## Runtime

```text
Browser
  |
  | HTTPS/WSS
  v
Apache :$PORT
  |                    same container
  +--> PHP ------------------------+
  |    Sessions/Lobbies/Accounts   |
  |    Join Tickets                v
  |                              SQLite
  |
  +--> /ws,/ping --> Go :8081
                      Lobby Actors
                      Queue/Reconnect
                      Match/Rules
                      Physics
                      |
                      +--> internal PHP persistence API
```

## Vertrauensgrenze

Three.js besitzt nur Darstellungszustand. Autoritative Gameplay-Entscheidungen liegen vollständig in Go.

Der Client darf insbesondere keine Kugelposition, Foulauswertung, Turn-Änderung oder Match-Ergebnis festlegen.

## PHP

PHP verantwortet:

- Guest-/Account-Sessions
- Lobby-Metadaten und private Lobby-Passwörter
- CSRF
- kurzlebige Join-JWTs
- Profil/History
- SQLite-Persistenz
- interne Persistenz-API für Go

## Go

Go verantwortet:

- WebSockets
- Participants und Connections
- Spectators
- FIFO-Rotation
- Ready/Reconnect
- Match-State-Machine
- Shot Validation
- Rules Engine
- 120-Hz-Physics
- Chat

## Apache

Apache ist der einzige öffentliche Netzwerklistener. Es liefert PHP/Assets aus und proxyt die Echtzeitendpunkte zu Go.

Damit benötigt Render nur einen Web Service und nur einen öffentlichen Port.

## Persistenz

Die Single-Service-Edition verwendet SQLite. Match-Persistenz wird nicht direkt aus Go in die Datei geschrieben, sondern über abgesicherte interne PHP-Endpunkte. Dadurch besitzt PHP exklusiv die SQL-Schreiblogik und Go bleibt unabhängig von SQLite-Treibern.

## Prozessmodell

Der Entrypoint startet und überwacht zwei Prozesse:

- Apache/PHP
- Go Game Server

Beendet sich einer unerwartet, beendet der Entrypoint den anderen ebenfalls, sodass die Plattform den Container sauber neu starten kann.
