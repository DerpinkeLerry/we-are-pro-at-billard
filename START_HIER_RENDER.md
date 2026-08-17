# Render in 60 Sekunden

Du brauchst **nur einen einzigen Web Service**.

1. Dieses Repository zu GitHub pushen.
2. Render öffnen → **New → Web Service**.
3. Dein GitHub-Repository auswählen.
4. Branch: `main`.
5. Runtime/Language: **Docker**.
6. Instance Type: **Free**.
7. Root Directory: **leer lassen**.
8. Keine Environment Variables anlegen.
9. **Create Web Service** klicken.

Fertig. Render findet den `Dockerfile` direkt im Repository-Root und startet PHP, Apache, SQLite und den Go-Game-Server gemeinsam.

Healthcheck optional: `/health`

## Free-Hinweis

Die App läuft vollständig als ein Service. Auf Render Free ist das lokale Dateisystem jedoch flüchtig. Deshalb können Accounts und Match-History nach Spin-down, Restart oder Redeploy verschwinden. Für dauerhaft gespeicherte Daten wäre später externe Persistenz nötig.
