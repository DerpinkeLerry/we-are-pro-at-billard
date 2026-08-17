# Physics und Tischgeometrie

## Einheiten und Integrator

- SI-Einheiten: Meter, Sekunde, Kilogramm, Radiant
- `float64` für serverseitigen Zustand
- Basisfrequenz: 120 Hz
- adaptive Substeps bis maximal 16 Teilsteps; eine Kugel bewegt sich dabei höchstens 25 % ihres Radius
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

- Sliding-Friction wirkt gegen die Slip-Richtung und verändert lineare sowie Winkelgeschwindigkeit mit dem Trägheitsmoment einer Vollkugel (`I = 2/5 mr²`).
- Der Zeitpunkt des Übergangs von Gleiten zu Rollen wird innerhalb des Teilsteps analytisch bestimmt. Geschwindigkeit und Rotation werden nicht mehr über einen künstlichen Blend angenähert.
- In der Rollphase bleiben lineare Geschwindigkeit und Rollrotation exakt auf der No-Slip-Bedingung gekoppelt; Rolling Resistance baut beide gemeinsam ab.
- Side Spin besitzt eigenen exponentiellen Decay.
- Sleep benötigt lineare und angulare Unterschreitung über eine Mindestdauer; ein einzelner niedriger Samplewert stoppt den Ball nicht.

Koeffizienten stehen ausschließlich in `config/physics/physics-v2.json`.

## Ball-Ball

- normaler Restitutionsimpuls mit Absenkung bei sehr kleinen Kontaktgeschwindigkeiten gegen Mikrobounces
- tangentialer Coulomb-Impuls mit korrekter effektiver Masse `7/m` für zwei Vollkugeln
- Side-Spin-Kopplung
- Penetrationskorrektur als numerische Stabilisierung
- mehrere Solver-Iterationen für Rack-/Kontaktgruppen
- swept circle TOI für High-Speed-Crossings

## Cushions und Jaws

Cushions und Pocket Jaws sind endliche Liniensegmente und werden mit demselben Kontaktmodell abgefragt. Tangentiale Cushion-Reibung koppelt Z-Spin an den Abprallwinkel. Der Tangentialimpuls ist durch den Normalimpuls begrenzt, damit ein flacher Streifkontakt nicht unphysikalisch viel Längsgeschwindigkeit vernichtet.

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
  | Ballzentrum überquert geometrische Throat-Linie mit einem vollen Kugelradius Abstand zu beiden Jaw-Endpunkten
  v
FALLING
  | gravity + liner/back-draft constraints
  +--> ON_TABLE      (flacher Rattle/Exit ist noch möglich)
  +--> POCKETED      (erst nach irreversiblem vertikalem Drop)
```

Beim Eintritt wird die XY-Position nicht zum Pocket-Center verschoben und es existiert keine anziehende Kraft. Mouth/Jaw-Kollisionen können den Ball zurück auf den Tisch schicken.

## Gemeinsame Geometriequelle

`config/table/pool-7ft-v2.json` ist die kanonische Quelle. Go erzeugt daraus Collision-Segmente/Pockets; Three.js erzeugt daraus sichtbare Rail-/Jaw-/Pocket-Geometrie und Development-Debug-Linien.

Aktuelle Baseline:

| Wert | Projektwert |
|---|---:|
| Playing Surface | 1.9812 × 0.9906 m (78 × 39 Zoll, 2:1) |
| Ball Diameter | 57.15 mm |
| Corner Mouth | 114.3 mm |
| Side Mouth | 127.0 mm |
| Corner Horizontal Cut | 142° |
| Side Horizontal Cut | 104° |
| Back Draft | 13.5° |
| Corner Shelf | 44.45 mm |
| Side Shelf | 6.35 mm |

Foot Spot und Head String liegen bei `±0.4953 m`, also exakt auf den Viertelpunkten der 7-ft-Spielfläche. Kugel-, Banden- und Taschenmaße werden nicht mit dem Tisch verkleinert: Ein 7-ft-Pooltisch verwendet dieselben 57,15-mm-Kugeln und regulären Pocket-Maße. Throat-Breiten werden aus Mouth, Shelf und Horizontal-Cut-Winkel validiert, nicht unabhängig geraten.

## Behobene Modellfehler in Physics v2

- Positiver vertikaler Cue-Offset erzeugt jetzt tatsächlich Follow/Topspin; zuvor war das Vorzeichen vertauscht.
- Ball-Ball-Reibung verwendet die Rotationsanteile beider Kugeln; der alte Nenner übertrug zu viel Seitenspin.
- Der Gleit-/Rollwechsel kann einen Integrationsschritt nicht mehr überschwingen.
- Bandenreibung folgt einem Coulomb-Limit statt bei jedem Kontakt pauschal einen festen Anteil der Tangentialgeschwindigkeit zu entfernen.
- Eine Tasche nimmt keine Kugel auf, die geometrisch noch einen Jaw-Endpunkt schneidet.
- Der Renderer integriert die übertragene 3D-Winkelgeschwindigkeit um ihre tatsächliche Weltachse statt X/Y zu vertauschen.

## Quellen

World Pool-Billiard Association:

- 2026 Rules of Play: `https://www.wpapool.com/wp-content/uploads/2026/01/2026.01.02-WPA-Rules.pdf`
- Recommended Equipment Specifications: `https://wpapool.com/wp-content/uploads/2024/01/RECOMMENDED-EQUIPMENT-SPECIFICATIONS.pdf`

7-ft-Spielfläche:

- Olhausen Table Dimensions (39 × 78 Zoll): `https://www.olhausenbilliards.com/wp-content/uploads/2021/03/Outside-Table-Dimensions_2019.pdf`

Die JSON-Konfiguration friert konkrete Projektwerte innerhalb der dokumentierten Bereiche ein, damit Simulation, Renderer und Match-Versionierung reproduzierbar bleiben.
