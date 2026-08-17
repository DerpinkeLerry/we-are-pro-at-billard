# Physics und Tischgeometrie

## Einheiten und Integrator

- SI-Einheiten: Meter, Sekunde, Kilogramm, Radiant
- `float64` für serverseitigen Zustand
- Basisfrequenz: 120 Hz
- adaptive Substeps anhand maximaler Ballverschiebung pro Schritt
- iterative Kontaktauflösung für Mehrfachkontakte
- swept Ball-Ball-Time-of-Impact für Paare, die einen Substep vollständig kreuzen würden
- maximaler Simulationshorizont pro Stoß: 25 Sekunden

Die Engine berechnet einen akzeptierten Stoß vollständig serverseitig voraus. Aus der Simulation werden ca. 30 Frames pro Simulationssekunde für Netzwerk-Playback extrahiert.

## Ballzustand

Jede Kugel besitzt:

- ID
- XY-Position und -Geschwindigkeit
- vollständige Winkelgeschwindigkeit `omega X/Y/Z`
- Höhe `Z` und vertikale Geschwindigkeit während Pocket-Fall
- State `table`, `falling`, `pocketed`, `off_table`
- Pocket-ID und Sleep-Hysterese

## Cue-Impact und Spin

Power wird in eine serverseitig begrenzte lineare Cue-Ball-Geschwindigkeit umgesetzt. Der Cue-Tip-Offset erzeugt echte Winkelgeschwindigkeit:

- positiver/negativer vertikaler Offset → Follow/Draw
- horizontaler Offset → Side Spin um Z

Cue-Skins ändern ausschließlich Darstellung.

## Tuchreibung

Am Ball/Tuch-Kontaktpunkt wird Schlupf aus linearer und angularer Geschwindigkeit berechnet.

- Sliding-Friction wirkt gegen die Slip-Richtung und verändert lineare sowie Winkelgeschwindigkeit.
- Nahe Rollbedingung wird auf kleinere Rolling Resistance gewechselt.
- Side Spin besitzt eigenen exponentiellen Decay.
- Sleep benötigt lineare und angulare Unterschreitung über eine Mindestdauer; ein einzelner niedriger Samplewert stoppt den Ball nicht.

Koeffizienten stehen ausschließlich in `config/physics/physics-v1.json`.

## Ball-Ball

- normaler Restitutionsimpuls
- tangentialer Impuls begrenzt durch Ballkontakt-Reibung
- Side-Spin-Kopplung
- Penetrationskorrektur als numerische Stabilisierung
- mehrere Solver-Iterationen für Rack-/Kontaktgruppen
- swept circle TOI für High-Speed-Crossings

## Cushions und Jaws

Cushions und Pocket Jaws sind endliche Liniensegmente und werden mit demselben Kontaktmodell abgefragt. Tangentiale Cushion-Reibung koppelt Z-Spin an den Abprallwinkel.

Die adaptive Substep-Grenze hält die maximale Bewegung bei regulären Cue-Geschwindigkeiten deutlich unter einem Ballradius; dadurch werden auch Segmentkontakte nicht nur auf groben 120-Hz-Endpunkten geprüft.

## Pocket-Modell

Es existiert kein `distance(ball, pocketCenter) < radius => pocketed`.

Jede Tasche besitzt:

- Mouth-Mittellinie und Mouth-Breite
- zwei Jaw-Segmente
- Throat-Mittellinie und daraus abgeleitete Throat-Breite
- Shelf-Tiefe
- Corner-/Side-spezifische Richtung und Form
- Vertical Back Draft
- Drop-Volumen/Liner

State-Übergang:

```text
ON_TABLE
  | Ballzentrum überquert geometrische Throat-Linie innerhalb der freien Breite
  v
FALLING
  | gravity + liner/back-draft constraints
  +--> ON_TABLE      (flacher Rattle/Exit ist noch möglich)
  +--> POCKETED      (erst nach irreversiblem vertikalem Drop)
```

Beim Eintritt wird die XY-Position nicht zum Pocket-Center verschoben und es existiert keine anziehende Kraft. Mouth/Jaw-Kollisionen können den Ball zurück auf den Tisch schicken.

## Gemeinsame Geometriequelle

`config/table/wpa-9ft-v1.json` ist die kanonische Quelle. Go erzeugt daraus Collision-Segmente/Pockets; Three.js erzeugt daraus sichtbare Rail-/Jaw-/Pocket-Geometrie und Development-Debug-Linien.

Aktuelle Baseline:

| Wert | Projektwert |
|---|---:|
| Playing Surface | 2.540 × 1.270 m |
| Ball Diameter | 57.15 mm |
| Corner Mouth | 114.3 mm |
| Side Mouth | 127.0 mm |
| Corner Horizontal Cut | 142° |
| Side Horizontal Cut | 104° |
| Back Draft | 13.5° |
| Corner Shelf | 44.45 mm |
| Side Shelf | 6.35 mm |

Throat-Breiten werden im Code aus Mouth, Shelf und Horizontal-Cut-Winkel abgeleitet, nicht unabhängig geraten.

## Quellen

World Pool-Billiard Association:

- 2026 Rules of Play: `https://www.wpapool.com/wp-content/uploads/2026/01/2026.01.02-WPA-Rules.pdf`
- Recommended Equipment Specifications: `https://wpapool.com/wp-content/uploads/2024/01/RECOMMENDED-EQUIPMENT-SPECIFICATIONS.pdf`

Die JSON-Konfiguration friert konkrete Projektwerte innerhalb der dokumentierten Bereiche ein, damit Simulation, Renderer und Match-Versionierung reproduzierbar bleiben.
