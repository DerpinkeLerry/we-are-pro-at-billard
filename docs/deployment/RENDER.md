# Render Deployment — Single Web Service

Diese Repository-Version ist für einen möglichst einfachen Render-Deploy gebaut.

## Benötigte Render-Ressourcen

Genau eine:

- **Web Service**

Nicht benötigt:

- kein separater Go-Service
- kein Render Postgres
- kein Key Value Service
- kein Blueprint
- keine manuell synchronisierten Secrets

## Einrichtung

1. `New` → `Web Service`
2. GitHub-Repository auswählen
3. Branch `main`
4. Runtime `Docker`
5. Root Directory leer
6. Root-`Dockerfile` verwenden
7. Free Instance auswählen, wenn gewünscht
8. Deploy

Der Docker-Entrypoint verwendet das von Render bereitgestellte `PORT` und startet Apache auf diesem öffentlichen Port. Go läuft im selben Container nur auf `127.0.0.1:8081`.

## Routing

```text
/                -> PHP/Three.js
/api/*           -> PHP
/health          -> PHP + Go readiness check
/ws              -> Apache reverse proxy -> Go
/ping            -> Apache reverse proxy -> Go
```

PHP erreicht Go intern über `http://127.0.0.1:8081`.

Go erreicht die internen PHP-Persistenzendpunkte über `http://127.0.0.1:$PORT`.

## Secrets

Falls nicht als Environment Variable gesetzt, erzeugt der Entrypoint bei jedem Containerstart automatisch:

- `JOIN_TOKEN_SECRET`
- `GAME_INTERNAL_SECRET`

Beide Prozesse erben dieselben Werte. Dadurch ist keine Render-Konfiguration nötig.

## SQLite

Standardpfad:

```text
/var/lib/pool/pool.sqlite
```

Migrationen laufen vor dem Start der Anwendung automatisch.

### Free Render

Ein kostenloser Render Web Service besitzt kein dauerhaftes lokales Dateisystem. SQLite-Daten sind deshalb für Test/Hobby-Betrieb geeignet, aber nicht als dauerhafte Production-Datenhaltung. Ein Spin-down, Restart oder Redeploy kann lokale Daten entfernen.

Für dauerhafte Accounts und Match-History muss später eine persistente externe Datenbank oder ein kostenpflichtiger Persistent Disk ergänzt werden.

## Healthcheck

Empfohlener Pfad:

```text
/health
```

Der Endpoint prüft:

- SQLite-Verfügbarkeit
- ob der interne Go-Prozess auf `/ping` antwortet

Er liefert `200`, wenn beide Komponenten verfügbar sind.

## Scaling

Nur eine Instanz verwenden. Der autoritative Lobby-/Match-State lebt im Go-Prozess. Horizontales Scaling erfordert vorher explizites Lobby-Sharding bzw. verteilte Actor-Ownership.
