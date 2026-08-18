# Implementation Status

Dieses Repository enthält die Single-Service-Edition des Pool-Arena-Projekts.

Enthalten:

- Root-`Dockerfile`, der PHP/Apache und Go in einem Image kombiniert
- Apache-WebSocket-Reverse-Proxy
- automatischer Render-`PORT`-Support
- automatisch generierte gemeinsame Secrets
- SQLite-Persistenz ohne externen Datenbankservice
- PHP Sessions/Accounts/Lobbies/Profile/History
- Go Lobby/Rotation/Reconnect/Rules/Physics/WebSocket
- interne HTTP-Persistenz zwischen Go und PHP
- Three.js Client mit 2D-/3D-Spielkamera und Grafikprofilen
- gemeinsame Table/Physics/Rules-Konfigurationen
- CI und Tests

Validierung beim Packaging:

- PHP-Dateien syntaktisch geprüft
- JavaScript-Dateien syntaktisch geprüft
- SQLite-Migrationsschema mit SQLite ausgeführt
- dependency-freie Go-Kernpakete (`physics`, `rules`, `match`, `persistence`) erfolgreich getestet
- vollständige Go-Suite ist in CI definiert; die Packaging-Umgebung selbst kann externe Go-Module von `proxy.golang.org` nicht herunterladen

Bekannte Hosting-Einschränkung:

- Auf Render Free ist das lokale Dateisystem flüchtig. Dadurch ist die eingebaute SQLite-Persistenz nicht dauerhaft über Spin-down/Restart/Redeploy hinweg.
