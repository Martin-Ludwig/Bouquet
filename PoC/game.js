(() => {
  'use strict';

  // ---------- Konfiguration (Balance-Regler) ----------
  // Karte: rechteckig, von Süden (Start) nach Norden (Ziel) in Gebiete geteilt.
  // Zwischen zwei Gebieten liegt eine Mauerreihe mit Durchgängen.
  const MAP_WIDTH = 13;         // Spalten
  const AREA_ROWS = 12;         // Reihen pro Gebiet
  const SEPARATOR_GAPS = [2, 2, 3]; // Durchgänge je Trennmauer, von Norden nach Süden; Gebiete = Mauern + 1
  const GAP_WIDTH = 2;          // Breite eines Durchgangs in Feldern
  const ZOOM = 1.2;               // Feldgröße relativ zu "ein Gebiet passt"; die Kamera folgt dem Spieler
  // TESTVERSION Ring-Zoom: Feldgröße nach Abstand zum Spieler (Index = Feldabstand, der letzte Wert gilt für alles weiter weg).
  // Spielerfeld 1x, erster Ring 0.9x, zweiter 0.8x, ab dem dritten 0.7x.
  const RING_SCALE = [1, 1, 1, 1];  // Ring-Zoom aus: Spielerfeld und alle Ringe 1x
  // TESTVERSION Höhen: jedes Feld hat eine Höhe 0-HEIGHT_MAX (zufällige Hügel, rund um den Start flach).
  // Höher heißt näher am Betrachter: Felder werden vom Spielerfeld aus um HEIGHT_K je Höhenstufe vergrößert,
  // und zwar relativ zur Höhe unter dem Spieler. Das Spielerfeld bleibt deshalb immer 1x.
  const HEIGHT_MAX = 3;
  let heightK = 0.07;
  const HILLS = [3, 6];          // Hügel pro Gebiet
  const HILL_RADIUS = [2.5, 5];  // in Feldern
  const FLAT_AT_START = 2;       // Radius um den Start ohne Höhe
  const PLAYER_MOVES = 2;       // Laufweite des Spielers; pro Zug eine Aktion: laufen oder angreifen
  const PLAYER_START_HP = 5;    // keine Obergrenze: Heilung in Städten kann darüber hinausgehen
  const RECRUIT_HP = 2;         // HP, die jede eigene Stadt genau einmal gibt
  // Die folgenden Anzahlen gelten pro Gebiet
  const CITY_COUNT = 6;
  const TOWER_COUNT = 3;
  const TOWER_RANGE = 8;        // Aussichtsturm: Sichtweite, sieht über Wälder und Mauern hinweg
  const TOWER_AT_WALL = 0.75;   // Chance, dass ein Turm an einer Mauer steht, über die man von ihm aus springen kann
  const WALL_SEGMENTS = 5;      // Mauerketten
  const WALL_LEN = [3, 5];
  const FOREST_BLOBS = 7;       // Waldflächen
  const FOREST_SIZE = [5, 9];
  // Wut: steigt jeden Zug, sinkt bei jedem Kill des Spielers. Je höher die Stufe, desto schneller kommt Nachschub.
  const RAGE_PER_TURN = 2;
  const RAGE_MAX = 40;
  const RAGE_KILL = 12;         // so viel Wut nimmt ein Kill weg
  const RAGE_LEVELS = [15, 25, 31]; // ab diesen Punkten Stufe 1, 2, 3 (darunter Stufe 0)
  const SPAWN_RATE = [0, 2, 3, 4];  // Nachschub-Zähler pro Zug je Stufe
  const SPAWN_AT = 10;          // erreicht der Zähler diesen Wert: ein Gegner (oder eine Armee) erscheint, Zähler = 0
  const SPAWN_MIN_ROWS = 5;     // Nachschub erscheint SPAWN_MIN_ROWS bis AREA_ROWS Reihen nördlich vom Spieler
  const HUNTER_AGGRO = 8;       // Jäger verfolgen den Spieler ab dieser Wegdistanz (und nur mit Sichtlinie)
  const RAIDER_AGGRO = 3;       // Räuber lassen von Städten ab, wenn der Spieler so nah und in Sicht ist
  const RAID_RANGE = 10;        // Räuber greifen nur Städte an, die höchstens so weit vom Spieler entfernt sind
  const WANDER_CHANCE = 0.3;    // sonst ziellos umherlaufen
  const STORM_DELAY = 1;        // ab dem Ende dieses Zugs läuft die Sturm-Welle (beginnt mit 0: eine Runde Schonzeit)
  // Vorrücken pro Zug in halben Reihen (2 = eine Hex-Reihe), als Welle wiederholt: mal Pause, mal schnell.
  // Im Schnitt eine halbe Reihe pro Zug.
  const STORM_WAVE = [0, 1, 2, 3, 2, 1];
  const STORM_DAMAGE = 1;       // HP-Verlust pro Zug im Sturm, für Spieler und Gegner
  const ARMY_AGGRO = 8;         // Armeen bemerken den Spieler ab dieser Wegdistanz (mit Sichtlinie)
  const ARMY_HOLD = 2;          // Abstand, den eine Armee hält, bis sie gemeinsam zuschlagen kann
  const ARMY_STRIKE = 2;        // so viele Mitglieder müssen gleichzeitig zuschlagen können
  const MIN_SPACING = 4;       // Mindestabstand zwischen Städten bzw. zwischen Türmen
  const PLACE_TRIES = 2000;     // Versuche pro zufälliger Platzierung
  const FOREST_TRIES = 200;     // Versuche, eine Waldfläche wachsen zu lassen
  const TERRAIN_TRIES = 50;     // so oft wird das Gelände neu gewürfelt, falls das Ziel unerreichbar ist
  const LOG_LINES = 8;

  // Gegnertypen
  //  target(e, fieldTo) sagt, wohin der Gegner will: { field, seen, orWander } oder null.
  //    field:    Distanzfeld zum Ziel (über fieldTo, damit gleiche Felder geteilt werden)
  //    seen:     Gegner sieht den Spieler und merkt sich seine Position
  //    orWander: kein Schritt möglich -> stattdessen ziellos laufen
  //  spawns:  kommt als Nachschub
  //  razes:   zerstört eine Stadt, wenn er sie betritt
  //  becomes: verwandelt sich danach in diesen Typ (mit dessen vollen HP)
  //  speed:   Schritte pro Zug
  //  radius:  Größe der Spielfigur (Anteil der Feldgröße)
  //  squad:   tritt als Armee aus so vielen Mitgliedern auf (count = Anzahl Armeen)
  //  plan(group, fieldTo): plant den Zug einer Gruppe (Armee oder einzelner Gegner); Standard: planSingle
  // Ein einzelner Gegner greift an, wenn er zu Beginn des Gegnerzugs neben dem Spieler steht.
  const ENEMY_TYPES = {
    // Räuber: zielt auf die nächste Stadt, die er sieht. Ist der Spieler nah und in Sicht
    // oder gibt es keine Städte mehr, verfolgt er stattdessen den Spieler.
    raider: {
      name: 'Marodeur', letter: 'M', hp: 2, color: '#c92a2a', count: 4, speed: 1, radius: 0.6,
      spawns: true, razes: true, becomes: 'warlord', target: raid
    },
    // Kriegsherr: entsteht aus einem Räuber, der eine Stadt zerstört hat. Verhält sich wie ein Räuber.
    warlord: {
      name: 'Kriegsmech', nameDative: 'Kriegsmech', letter: 'K', hp: 5, color: '#e8590c', count: 0, speed: 2, radius: 0.72,
      spawns: false, razes: true, target: raid
    },
    // Jäger: verfolgt den Spieler, wenn er ihn sieht, sonst seine letzte bekannte Position. Ignoriert Städte.
    hunter: {
      name: 'Jagdbot', letter: 'J', hp: 3, color: '#9c36b5', count: 5, speed: 1, radius: 0.6,
      spawns: true, razes: false, target: (e, fieldTo) => chase(e, fieldTo, HUNTER_AGGRO)
    },
    // Soldat: Mitglied einer Armee. Die Armee läuft zusammen, hält Abstand und schlägt erst gemeinsam zu.
    soldier: {
      name: 'Trooper', letter: 'T', hp: 1, color: '#0c8599', count: 2, squad: 3, speed: 1, radius: 0.45,
      spawns: true, razes: false, plan: planArmy
    }
  };
  const SPAWN_TYPES = Object.keys(ENEMY_TYPES).filter((k) => ENEMY_TYPES[k].spawns);

  const COL = {
    fog: '#05070a',
    plain1: '#2e4a33',
    plain2: '#2a4430',
    forest: '#22452b',
    tree: '#143020',
    wall: '#77776e',
    wallTop: '#5f5f57',
    reach: '#3f7f4f',
    dim: 'rgba(5,8,12,0.55)',
    intent: '#ff8787',
    wander: '#adb5bd',
    neutral: '#adb5bd',
    threat: '#ff6b6b',
    highlight: '#ffe066',
    storm: 'rgba(110,130,175,0.6)',
    stormNext: 'rgba(110,130,175,0.25)',
    stormHatch: 'rgba(190,205,240,0.5)',
    stormNextHatch: 'rgba(190,205,240,0.2)',
    owned: '#1c7ed6',
    player: '#1864ab'
  };

  // Axiale Nachbarn für Hexfelder (Flat-Top)
  const DIRS = [[1, 0], [1, -1], [0, -1], [-1, 0], [-1, 1], [0, 1]];
  const SQRT3 = Math.sqrt(3);
  const HEX_CORNERS = DIRS.map((_, i) => [Math.cos(Math.PI / 3 * i), Math.sin(Math.PI / 3 * i)]);

  // ---------- DOM ----------
  const el = (id) => document.getElementById(id);
  const canvas = el('board');
  const view = canvas.getContext('2d');
  // Das Spielfeld wird in einen Puffer gezeichnet; Hover kopiert nur den Puffer und zeichnet den Rahmen.
  const buffer = document.createElement('canvas');
  const ctx = buffer.getContext('2d');
  const ui = {
    turn: el('turn'), hp: el('hp'), cities: el('cities'), rage: el('rage'), rageBar: el('rageBar'),
    endBtn: el('endBtn'), msg: el('msg'), log: el('log')
  };
  // Wut-Balken: grün unter Stufe 1, gelb ab Stufe 1, rot ab Stufe 2
  Object.assign(ui.rageBar, { max: RAGE_MAX, low: RAGE_LEVELS[0], high: RAGE_LEVELS[1], optimum: 0 });

  // ---------- Zustand ----------
  const AREA_COUNT = SEPARATOR_GAPS.length + 1;
  const MAP_ROWS = AREA_COUNT * AREA_ROWS + SEPARATOR_GAPS.length;
  let size = 0;                // 0, solange layout() noch nicht lief oder das Canvas keine Fläche hat; beim Zeichnen eines Feldes kurz dessen Größe
  let baseSize = 0;            // Feldgröße bei Faktor 1
  let ox = 0, oy = 0;
  let tiles = [];               // alle Felder; t.i ist der Index, t.nb die Nachbarfelder
  let grid = [];                // Raster MAP_ROWS x MAP_WIDTH -> Feld
  let player, enemies;
  let cityTiles = [], towerTiles = [];
  let turn, over, hover;
  let captured = 0;             // eingenommene Städte
  let rage = 0, spawnCounter = 0;
  let nextArmyId = 0;
  let stormFront = 0;           // Felder mit stormDepth > stormFront liegen im Sturm
  let reach = null;             // Distanzfeld der Spielerbewegung (null, wenn nicht am Zug)
  let visible, explored;        // Uint8Array je Feld: aktuell sichtbar / jemals gesehen
  let sightCache = new Map();   // Feldindex -> Indizes der von dort sichtbaren Felder
  let intents = [];             // vorhergesagte Gegnerzüge
  let intentsSeenAt = null;     // Spielerfeld, als der Gegnerplan berechnet wurde

  // ---------- Koordinaten & Distanz ----------
  const rand = (a, b) => a + Math.floor(Math.random() * (b - a + 1));
  const pick = (arr) => arr[rand(0, arr.length - 1)];
  const dist = (a, b) => {
    const dq = a.q - b.q, dr = a.r - b.r;
    return (Math.abs(dq) + Math.abs(dr) + Math.abs(dq + dr)) / 2;
  };
  const samePos = (a, b) => a.q === b.q && a.r === b.r;
  // Versatz-Koordinaten (Spalte, Reihe) <-> axial (q, r); ungerade Spalten liegen eine halbe Reihe tiefer
  const toAxial = (col, row) => ({ q: col, r: row - (col - (col & 1)) / 2 });
  const tileAt = (q, r) => {
    if (q < 0 || q >= MAP_WIDTH) return undefined;
    const row = r + (q - (q & 1)) / 2;
    return row < 0 || row >= MAP_ROWS ? undefined : grid[row * MAP_WIDTH + q];
  };
  const here = (p) => tileAt(p.q, p.r);
  const enemyAt = (t) => enemies.find((e) => samePos(e, t));
  // Pixelversatz eines Feldes relativ zum Kartenursprung
  const hexOffsetX = (q) => size * 1.5 * q;
  const hexOffsetY = (q, r) => size * SQRT3 * (r + q / 2);

  // Start ganz im Süden, Ziel ganz im Norden, beide mittig
  const START = toAxial(MAP_WIDTH >> 1, MAP_ROWS - 1);
  const GOAL = toAxial(MAP_WIDTH >> 1, 0);
  // Für Gelände und Bauten: kein Trennmauer-Feld und etwas Abstand zu Start und Ziel
  const farFromEnds = (p) => p.area !== -1 && dist(p, START) >= 3 && dist(p, GOAL) >= 2;

  // Feld frei für Spieler/Gegner? (keine Mauer, kein Gegner, kein Spieler)
  const isFree = (t) => !!t && !t.blocked && !enemyAt(t) && !samePos(player, t);

  // Sprungziele von t aus: gerade über genau ein Mauerfeld auf das Feld dahinter (das selbst keine Mauer ist).
  // Gilt nur, wenn man auf einem Aussichtsturm steht.
  const jumpTargets = (t) => DIRS
    .filter(([dq, dr]) => tileAt(t.q + dq, t.r + dr)?.blocked)
    .map(([dq, dr]) => tileAt(t.q + 2 * dq, t.r + 2 * dr))
    .filter((land) => land && !land.blocked);

  const isIntact = (t) => !!t.city && t.city.state !== 'ruin';
  const intactCities = () => cityTiles.filter(isIntact);
  const inReach = (t) => !!reach && reach[t.i] > 0;

  // Sturm: Tiefe nach Süden in halben Hex-Reihen (passt zur Pixel-Höhe r + q/2; = 2 * Reihe + Spalte ungerade)
  const stormDepth = (t) => 2 * t.r + t.q;
  const MAX_STORM_DEPTH = 2 * (MAP_ROWS - 1) + 1;
  const inStorm = (t) => stormDepth(t) > stormFront;
  // Vorrücken am Ende von Zug t
  const stormStep = (t) => t >= STORM_DELAY ? STORM_WAVE[(t - STORM_DELAY) % STORM_WAVE.length] : 0;
  // Front nach dem Ende des aktuellen Zugs (für die Vorschau)
  const nextStormFront = () => stormFront - stormStep(turn);

  // ---------- Sicht ----------
  // Sichtlinie frei? Nur die Felder dazwischen zählen. Mauern und Wälder blockieren.
  // Kleine Verschiebung, damit Randfälle eindeutig sind.
  function lineClear(a, b) {
    const n = dist(a, b);
    for (let i = 1; i < n; i++) {
      const s = i / n;
      const m = here(cubeRound(a.q + (b.q - a.q) * s + 1e-6, a.r + (b.r - a.r) * s + 2e-6));
      if (m && (m.blocked || m.forest)) return false;
    }
    return true;
  }

  // Felder, die man von t aus sieht. Mauern und Wälder ändern sich im Spiel nicht,
  // daher wird das pro Standort nur einmal berechnet und gemerkt (sightCache, geleert in init).
  function sightFrom(t) {
    let seen = sightCache.get(t.i);
    if (!seen) {
      seen = tiles.filter((x) => lineClear(t, x)).map((x) => x.i);
      sightCache.set(t.i, seen);
    }
    return seen;
  }

  // Spieler: unbegrenzte Reichweite, aber Hindernisse blockieren.
  // Eigene Türme: feste Reichweite, Hindernisse egal (tower.sight wird beim Aufbau berechnet).
  function updateVision() {
    visible = new Uint8Array(tiles.length);
    for (const i of sightFrom(here(player))) visible[i] = 1;
    for (const t of towerTiles) {
      if (t.tower.owner !== 'player') continue;
      for (const i of t.tower.sight) visible[i] = 1;
    }
    // Was der Spieler zuletzt gesehen hat (für Städte und Türme im Nebel)
    for (const t of tiles) {
      if (!visible[t.i]) continue;
      explored[t.i] = 1;
      t.seenCity = t.city ? t.city.state : null;
      t.seenTower = t.tower ? t.tower.owner : null;
    }
  }

  // ---------- Distanzfeld & Gegner-Logik ----------
  // Breitensuche von einer oder mehreren Quellen. Jedes Feld bekommt die Wegdistanz (-1 = unerreichbar).
  // passable entscheidet, welche Felder betreten werden; maxDepth begrenzt die Suche.
  function distanceField(sources, passable = (t) => !t.blocked, maxDepth = Infinity) {
    const field = new Int16Array(tiles.length).fill(-1);
    const queue = [];
    for (const s of sources) {
      const t = here(s);
      field[t.i] = 0;
      queue.push(t);
    }
    for (let i = 0; i < queue.length; i++) {
      const cur = queue[i];
      const d = field[cur.i];
      if (d >= maxDepth) continue;
      for (const n of cur.nb) {
        if (field[n.i] !== -1 || !passable(n)) continue;
        field[n.i] = d + 1;
        queue.push(n);
      }
    }
    return field;
  }

  // Ein Schritt bergab im Distanzfeld, falls ein freies Nachbarfeld näher liegt
  function stepByField(e, field) {
    const cur = here(e);
    let best = null, bestVal = field[cur.i];
    if (bestVal === -1) return null;
    for (const n of cur.nb) {
      const v = field[n.i];
      if (v !== -1 && v < bestVal && isFree(n)) { bestVal = v; best = n; }
    }
    return best;
  }

  // Sieht e den Spieler? Höchstens range Felder Weg (laut Distanzfeld zum Spieler) und freie Sichtlinie.
  function seesPlayer(e, playerField, range) {
    const d = playerField[here(e).i];
    return d !== -1 && d <= range && lineClear(e, player);
  }

  // Zufälliger Schritt auf ein freies Nachbarfeld (mit WANDER_CHANCE), sonst null
  function wanderStep(e) {
    if (Math.random() >= WANDER_CHANCE) return null;
    const opts = here(e).nb.filter(isFree);
    return opts.length ? pick(opts) : null;
  }

  // Verfolgt den Spieler, wenn er höchstens range Felder Weg entfernt und in Sicht ist,
  // sonst seine letzte bekannte Position.
  function chase(e, fieldTo, range) {
    const field = fieldTo([here(player)]);
    if (seesPlayer(e, field, range)) return { field, seen: true };
    if (e.lastSeen && !samePos(e, e.lastSeen)) return { field: fieldTo([e.lastSeen]), orWander: true };
    return null;
  }

  // Räuber-Verhalten: Spieler in der Nähe vor Städten; ohne Städte nur noch Jagd auf den Spieler.
  function raid(e, fieldTo) {
    const cities = intactCities();
    if (!cities.length) return chase(e, fieldTo, HUNTER_AGGRO);
    const near = chase(e, fieldTo, RAIDER_AGGRO);
    if (near && near.seen) return near;
    // Nur Städte in der Nähe des Spielers, damit er die Zerstörung miterlebt und verhindern kann
    const inSight = cities.filter((c) => dist(c, player) <= RAID_RANGE && lineClear(e, c));
    return inSight.length ? { field: fieldTo(inSight) } : near;
  }

  // Entscheidet den nächsten Schritt, ohne etwas zu verändern.
  // random = true heißt: der Schritt ist zufällig, also nicht vorhersagbar.
  function decideMove(e, fieldTo) {
    const aim = ENEMY_TYPES[e.type].target(e, fieldTo);
    if (aim) {
      const to = stepByField(e, aim.field);
      if (to || !aim.orWander) return { to, random: false, seen: !!aim.seen };
    }
    const to = wanderStep(e);
    return { to, random: !!to, seen: false };
  }

  // Wirkung eines Gegnerschritts, gleich für Vorschau und echten Zug.
  // Gibt zurück, ob dabei eine Stadt zerstört wurde.
  function applyMove(e, to) {
    e.q = to.q;
    e.r = to.r;
    const type = ENEMY_TYPES[e.type];
    if (!type.razes || !isIntact(to)) return false;
    to.city.state = 'ruin';
    if (type.becomes) {
      e.type = type.becomes;
      e.hp = ENEMY_TYPES[type.becomes].hp;
    }
    return true;
  }

  // Alles, was applyMove verändert. Die Vorschau stellt damit den Zustand wieder her.
  function snapshot() {
    const saved = enemies.map((e) => [e, e.q, e.r, e.type, e.hp]);
    const cities = cityTiles.map((t) => [t, t.city.state]);
    return () => {
      for (const [e, q, r, type, hp] of saved) Object.assign(e, { q, r, type, hp });
      for (const [t, state] of cities) t.city.state = state;
    };
  }

  // Plan für den Gegnerzug, berechnet zu Beginn des Spielerzugs und als Pfeile angezeigt.
  // Genau dieser Plan wird nach der Aktion des Spielers ausgeführt; die Gegner reagieren also nicht mehr darauf.
  // Ein Eintrag: { enemy, steps: [{ to, wander, destroys }], attack, seen }
  //  attack: greift nach den Schritten an, falls der Spieler dann noch daneben steht
  //  seen:   der Gegner hat den Spieler beim Planen gesehen (dort, wo intentsSeenAt steht)
  // Die Gegner werden gruppenweise (Armee oder einzeln) nacheinander simuliert, damit sie sich
  // gegenseitig blockieren. Zufällige Schritte werden hier schon ausgewürfelt.
  function computeIntents() {
    if (over) return [];
    intentsSeenAt = here(player);
    const restore = snapshot();
    // Distanzfelder pro Zielmenge nur einmal berechnen
    const fields = new Map();
    const fieldTo = (targets) => {
      const k = targets.map((t) => t.i).join();
      if (!fields.has(k)) fields.set(k, distanceField(targets));
      return fields.get(k);
    };
    // Gruppen in einem Durchgang bilden: jede Armee zusammen, sonst jeder Gegner für sich
    const groups = new Map();
    for (const e of enemies) {
      const k = e.army ? 'army' + e.army : e;
      if (!groups.has(k)) groups.set(k, []);
      groups.get(k).push(e);
    }
    const out = [];
    for (const group of groups.values()) {
      const plan = ENEMY_TYPES[group[0].type].plan || planSingle;
      out.push(...plan(group, fieldTo));
    }
    restore();
    return out;
  }

  // Einzelner Gegner: steht er neben dem Spieler, greift er an, sonst bis zu speed Schritte.
  // (Der Spieler bewegt sich in der Simulation nicht, die Nachbarschaft gilt also ab Zugbeginn.)
  function planSingle([e], fieldTo) {
    if (dist(e, player) === 1) return [{ enemy: e, attack: true, seen: true, steps: [] }];
    // Schrittzahl vom Typ zu Zugbeginn: Ein Räuber, der sich verwandelt, läuft erst nächsten Zug schneller
    const speed = ENEMY_TYPES[e.type].speed;
    const steps = [];
    let seen = false;
    for (let s = 0; s < speed; s++) {
      const move = decideMove(e, fieldTo);
      seen = seen || move.seen;
      if (!move.to) break;
      // Simuliert: Eine zerstörte Stadt ist für die folgenden Gegner schon Ruine
      steps.push({ to: move.to, wander: move.random, destroys: applyMove(e, move.to) });
    }
    return [{ enemy: e, attack: false, seen, steps }];
  }

  // Armee: Alle Mitglieder werden gemeinsam geplant. Sie halten ARMY_HOLD Felder Weg Abstand,
  // bis mindestens ARMY_STRIKE von ihnen im selben Zug neben den Spieler kommen.
  // Dann stürmen diese heran und schlagen gemeinsam zu.
  function planArmy(members, fieldTo) {
    const pf = fieldTo([here(player)]);
    const d = (m) => pf[here(m).i];
    const sees = members.some((m) => seesPlayer(m, pf, ARMY_AGGRO));
    const plans = new Map(members.map((m) => [m, { enemy: m, attack: false, seen: sees, steps: [] }]));
    // Ausführungsreihenfolge = Simulationsreihenfolge, sonst blockieren sich Mitglieder anders als angezeigt
    const order = [];
    const step = (m, to, wander = false) => {
      if (!order.includes(m)) order.push(m);
      plans.get(m).steps.push({ to, wander, destroys: applyMove(m, to) });
    };
    const result = () => [...order, ...members.filter((m) => !order.includes(m))].map((m) => plans.get(m));

    if (sees) {
      // Probelauf: Wer kommt diesen Zug wirklich neben den Spieler?
      const ready = members.filter((m) => d(m) !== -1 && d(m) <= ARMY_HOLD);
      const restore = snapshot();
      const strikers = [];
      for (const m of ready) {
        const to = d(m) === 1 ? null : stepByField(m, pf);
        if (d(m) !== 1 && !to) continue;
        if (to) applyMove(m, to);
        strikers.push([m, to]);
      }
      restore();

      if (strikers.length >= ARMY_STRIKE) {
        for (const [m, to] of strikers) {
          if (to) step(m, to);
          plans.get(m).attack = true;
        }
        for (const m of members) {
          if (plans.get(m).attack) continue;
          const to = stepByField(m, pf);
          if (to) step(m, to);
        }
      } else {
        // Abstand halten: von weiter weg herankommen, aus der Nähe zurückweichen
        for (const m of members) {
          let to = null;
          if (d(m) > ARMY_HOLD) to = stepByField(m, pf);
          else if (d(m) !== -1 && d(m) < ARMY_HOLD) to = here(m).nb.find((n) => pf[n.i] === ARMY_HOLD && isFree(n));
          if (to) step(m, to);
        }
      }
      return result();
    }

    // Spieler nicht in Sicht: gemeinsame Erinnerung, sonst zusammenbleiben
    const memory = members.map((m) => m.lastSeen).find(Boolean);
    if (memory && !members.some((m) => samePos(m, memory))) {
      const field = fieldTo([memory]);
      for (const m of members) {
        const to = stepByField(m, field);
        if (to) step(m, to);
      }
    } else {
      // Anführer wandert, die anderen bleiben bei ihm
      const [leader, ...rest] = members;
      const wander = wanderStep(leader);
      if (wander) step(leader, wander, true);
      const lf = fieldTo([here(leader)]);
      for (const m of rest) {
        if (dist(m, leader) <= 1) continue;
        const to = stepByField(m, lf);
        if (to) step(m, to);
      }
    }
    return result();
  }

  // Die Gruppe eines Gegners: seine Armee oder er allein
  const squadOf = (e) => e.army ? enemies.filter((m) => m.army === e.army) : [e];

  // Führt den Plan genau so aus, wie er angezeigt wurde
  function executePlan(plan) {
    for (const p of plan) {
      if (over) break;
      const e = p.enemy;
      if (!enemies.includes(e)) continue; // vom Spieler besiegt

      // Sieht er den Spieler noch, merkt er sich die neue Position, sonst die beim Planen
      if (p.seen) e.lastSeen = lineClear(e, player) ? here(player) : intentsSeenAt;
      let attacks = p.attack;
      for (const s of p.steps) {
        // Steht der Spieler im Weg, bleibt der Gegner stehen und greift ihn dort an
        if (samePos(player, s.to)) {
          attacks = true;
          break;
        }
        if (!isFree(s.to)) break;
        const before = e.type;
        if (applyMove(e, s.to)) {
          log('Ein ' + ENEMY_TYPES[before].name + ' zerstört eine Kolonie!' +
            (e.type !== before ? ' Er wird zum ' + ENEMY_TYPES[e.type].nameDative + '.' : ''));
        }
        // Angekommen: Erinnerung löschen (für die ganze Gruppe), dann wieder ziellos
        if (e.lastSeen && samePos(e, e.lastSeen)) for (const m of squadOf(e)) m.lastSeen = null;
      }

      // Angriff mit voller Kraft: Schaden = eigene HP, kein Gegenschlag.
      // Nur wenn der Spieler noch daneben steht; ist er ausgewichen, geht der Angriff ins Leere.
      if (attacks && dist(e, player) === 1) {
        damagePlayer(e.hp, 'Ein ' + ENEMY_TYPES[e.type].name + ' greift dich an: -' + e.hp + ' HP.', 'Du wurdest besiegt.');
      }
    }
  }

  function damagePlayer(amount, hitMsg, deathMsg) {
    player.hp -= amount;
    log(hitMsg);
    if (player.hp > 0) return;
    player.hp = 0;
    over = 'lose';
    log(deathMsg);
  }

  // Schaden an einem Gegner; stirbt er, verschwindet er. Nur Kills des Spielers senken die Wut.
  // Gibt zurück, ob er gestorben ist.
  function damageEnemy(e, amount, byPlayer) {
    e.hp -= amount;
    if (e.hp > 0) return false;
    enemies = enemies.filter((x) => x !== e);
    if (byPlayer) rage = Math.max(0, rage - RAGE_KILL);
    return true;
  }

  // Setzt einen Gegner auf ein freies Feld, das ok erfüllt; Armeen als Trupp auf benachbarten Feldern.
  // Gibt zurück, ob es geklappt hat.
  function placeEnemy(typeId, ok) {
    const cfg = ENEMY_TYPES[typeId];
    const t = randomFreeTile(ok);
    if (!t) return false;
    const army = cfg.squad ? ++nextArmyId : 0;
    const spawn = (p) => enemies.push({ q: p.q, r: p.r, type: typeId, hp: cfg.hp, army });
    spawn(t);
    const placed = [t];
    while (placed.length < (cfg.squad || 1)) {
      const opts = placed.flatMap((p) => p.nb).filter((n) => isFree(n) && ok(n));
      if (!opts.length) break;
      const n = pick(opts);
      spawn(n);
      placed.push(n);
    }
    return true;
  }

  const rageLevel = () => RAGE_LEVELS.filter((min) => rage >= min).length;

  // Am Zugende: Wut steigt, je nach Stufe läuft der Nachschub-Zähler
  function updateRage() {
    rage = Math.min(RAGE_MAX, rage + RAGE_PER_TURN);
    spawnCounter += SPAWN_RATE[rageLevel()];
    // Zähler nur zurücksetzen, wenn wirklich jemand erschienen ist; sonst nächsten Zug erneut versuchen
    if (spawnCounter >= SPAWN_AT && spawnEnemy()) spawnCounter = 0;
  }

  // Nachschub bevorzugt nördlich vom Spieler; ist dort kein Platz (z. B. im nördlichsten Gebiet),
  // irgendwo mit Abstand außerhalb des Sturms. Gibt zurück, ob jemand erschienen ist.
  function spawnEnemy() {
    if (!SPAWN_TYPES.length) return false;
    const row = here(player).row;
    const type = pick(SPAWN_TYPES);
    const cfg = ENEMY_TYPES[type];
    const what = cfg.squad ? 'ein Trupp' : 'ein ' + cfg.name;
    if (placeEnemy(type, (p) => p.row <= row - SPAWN_MIN_ROWS && p.row >= row - AREA_ROWS && dist(p, player) >= 4)) {
      log('Landung: ' + what + ' erscheint im Norden.');
      return true;
    }
    if (placeEnemy(type, (p) => p.area !== -1 && dist(p, player) >= SPAWN_MIN_ROWS && !inStorm(p))) {
      log('Landung: ' + what + ' erscheint.');
      return true;
    }
    return false;
  }

  // Zufälliges freies Feld, das filter erfüllt (der billige Filter zuerst, isFree durchsucht die Gegner)
  function randomFreeTile(filter) {
    for (let i = 0; i < PLACE_TRIES; i++) {
      const t = pick(tiles);
      if (filter(t) && isFree(t)) return t;
    }
    return null;
  }

  // ---------- Ring-Zoom (verzerrt) ----------
  // Die ganze Ebene wird radial gestreckt: ein Punkt im Abstand H (in Feldschritten, hexförmig gemessen) vom Spielerfeld
  // rückt auf G(H). G ist das Integral der Feldgröße; zwischen den Ringen wird die Größe weich übergeblendet.
  // Alle Felder werden aus verschobenen Ecken gezeichnet, deshalb berühren sich ihre Kanten weiterhin
  // (die Felder sind dabei verzerrt, keine regelmäßigen Sechsecke mehr).
  const scaleAt = (h) => {
    const a = Math.min(Math.floor(h), RING_SCALE.length - 1), b = Math.min(a + 1, RING_SCALE.length - 1);
    return RING_SCALE[a] + (RING_SCALE[b] - RING_SCALE[a]) * Math.min(1, h - Math.floor(h));
  };
  const G_STEP = 0.05;
  const gTable = [0];
  const warpG = (h) => {
    const k = h / G_STEP, i = Math.floor(k);
    while (gTable.length <= i + 1) gTable.push(gTable[gTable.length - 1] + scaleAt((gTable.length - 0.5) * G_STEP) * G_STEP);
    return gTable[i] + (gTable[i + 1] - gTable[i]) * (k - i);
  };
  // Hexförmiger Abstand eines Punktes (Einheit: Hex-Radius) in Feldschritten
  const hexNorm = (x, y) => {
    const q = 2 / 3 * x, r = -x / 3 + SQRT3 / 3 * y;
    return Math.max(Math.abs(q), Math.abs(r), Math.abs(q + r));
  };
  const warp = (x, y) => {
    const h = hexNorm(x, y);
    const f = h < 1e-9 ? scaleAt(0) : warpG(h) / h;
    return { x: x * f, y: y * f };
  };
  // Größe, mit der Figuren (Kreise, Schrift) auf einem Feld gezeichnet werden, und das aktuelle Feld für hexPath
  let curTile = null;
  function atScale(t, fn) {
    size = baseSize * (t.sc || 1);
    curTile = t;
    fn();
    curTile = null;
    size = baseSize;
  }

  // ---------- Pixel <-> Hex ----------
  // Feld unter einem Pixel: das Feld, dessen (verzerrtes) Sechseck den Punkt enthält
  function toHex(px, py) {
    for (const t of tiles) {
      if (!t.pts || Math.hypot(px - t.x, py - t.y) > baseSize * 2.5) continue;
      let inside = false;
      for (let a = 0, b = 5; a < 6; b = a++) {
        const [xa, ya] = t.pts[a], [xb, yb] = t.pts[b];
        if ((ya > py) !== (yb > py) && px < (xb - xa) * (py - ya) / (yb - ya) + xa) inside = !inside;
      }
      if (inside) return t;
    }
    return { q: -999, r: -999 };
  }

  function cubeRound(fq, fr) {
    const fs = -fq - fr;
    let q = Math.round(fq), r = Math.round(fr), s = Math.round(fs);
    const dq = Math.abs(q - fq), dr = Math.abs(r - fr), ds = Math.abs(s - fs);
    if (dq > dr && dq > ds) q = -r - s;
    else if (dr > ds) r = -q - s;
    return { q, r };
  }

  // ---------- Layout ----------
  // Pixelmittelpunkte der Felder; ändern sich nur mit size/ox/oy
  // Kamera: zentriert auf den Spieler, aber nicht über den Kartenrand hinaus.
  // Ist die Karte in einer Richtung kleiner als die Ansicht, wird sie dort mittig gezeigt.
  // Die Karte reicht relativ zum Ursprung o von lo bis hi.
  const clampCam = (o, viewLen, lo, hi) =>
    hi - lo <= viewLen ? (viewLen - lo - hi) / 2 : Math.min(-lo, Math.max(viewLen - hi, o));

  // Das Spielerfeld liegt in der Bildmitte; alle Ecken werden durch den Ring-Zoom verschoben.
  // Am Kartenrand bleibt die Ansicht leer (keine Begrenzung der Kamera).
  function placeTiles() {
    const w = canvas.clientWidth, h = canvas.clientHeight;
    const p = here(player);
    ox = 0; oy = 0; // Ankerpunkt der Sturmschraffur: fest am Bildschirm
    const base = p.vh ? p.vh.reduce((a, v) => a + v, 0) / 6 : 0; // Höhe unter dem Spieler
    for (const t of tiles) {
      const cx = 1.5 * (t.q - p.q), cy = SQRT3 * ((t.r - p.r) + (t.q - p.q) / 2);
      let sum = 0;
      t.pts = HEX_CORNERS.map(([dx, dy], k) => {
        const c = warp(cx + dx, cy + dy);
        const f = Math.max(0.3, 1 + heightK * ((t.vh ? t.vh[k] : 0) - base));
        sum += f;
        return [w / 2 + c.x * f * baseSize, h / 2 + c.y * f * baseSize];
      });
      t.sc = scaleAt(dist(t, p)) * sum / 6;
      t.x = t.pts.reduce((a, c) => a + c[0], 0) / 6;
      t.y = t.pts.reduce((a, c) => a + c[1], 0) / 6;
    }
  }

  // Felder im sichtbaren Ausschnitt (ganze Reihen, eine Reihe Rand); tiles ist reihenweise sortiert
  function tilesOnScreen() {
    const w = canvas.clientWidth, h = canvas.clientHeight, m = baseSize * 2.5;
    return tiles.filter((t) => t.x > -m && t.x < w + m && t.y > -m && t.y < h + m);
  }

  function layout() {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight;
    for (const c of [canvas, buffer]) {
      c.width = w * dpr;
      c.height = h * dpr;
    }
    view.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    // Größe, bei der ein ganzes Gebiet passt, mal ZOOM
    size = baseSize = Math.min(w / (1.5 * (MAP_WIDTH - 1) + 2), h / (SQRT3 * (AREA_ROWS + 0.5))) * 0.95 * ZOOM;
    if (player) show(); // in der Weltkarte läuft noch kein Kampf, bis HexGame.start kommt
  }

  // ---------- Zeichnen ----------
  // Sechseck um (cx, cy) mit Größe s. Innerhalb von atScale: die verzerrten Ecken des Feldes, um dessen Mitte auf s skaliert.
  function hexPath(cx, cy, s = size, g = ctx) {
    g.beginPath();
    const t = curTile;
    const k = t ? s / (baseSize * t.sc) : 0;
    for (let i = 0; i < 6; i++) {
      const x = t ? cx + (t.pts[i][0] - t.x) * k : cx + s * HEX_CORNERS[i][0];
      const y = t ? cy + (t.pts[i][1] - t.y) * k : cy + s * HEX_CORNERS[i][1];
      if (i === 0) g.moveTo(x, y); else g.lineTo(x, y);
    }
    g.closePath();
  }

  function fillHex(x, y, color, s = size) {
    hexPath(x, y, s);
    ctx.fillStyle = color;
    ctx.fill();
  }

  // Schraffur für den Sturm, damit er nicht nur an der Farbe erkennbar ist.
  // Die Kachel wird in Gerätepixeln gezeichnet (scharf bei jedem dpr) und an der Karte verankert,
  // damit die Linien beim Scrollen der Kamera nicht schwimmen.
  const hatchTiles = new Map();
  function hatchPattern(color) {
    const dpr = window.devicePixelRatio || 1;
    const k = color + '|' + dpr;
    if (!hatchTiles.has(k)) {
      const n = Math.round(10 * dpr);
      const tile = document.createElement('canvas');
      tile.width = tile.height = n;
      const g = tile.getContext('2d');
      g.strokeStyle = color;
      g.lineWidth = Math.max(1, 1.5 * dpr);
      g.beginPath();
      // Drei parallele Diagonalen, damit die Kachel nahtlos anschließt
      g.moveTo(0, n); g.lineTo(n, 0);
      g.moveTo(-n, n); g.lineTo(0, 0);
      g.moveTo(0, 2 * n); g.lineTo(n, n);
      g.stroke();
      hatchTiles.set(k, tile);
    }
    const p = ctx.createPattern(hatchTiles.get(k), 'repeat');
    p.setTransform(new DOMMatrix().translate(ox, oy).scale(1 / dpr));
    return p;
  }

  function hatchHex(x, y, pattern) {
    hexPath(x, y);
    ctx.fillStyle = pattern;
    ctx.fill();
  }

  function strokeHex(x, y, color, width, g = ctx) {
    hexPath(x, y, size, g);
    g.strokeStyle = color;
    g.lineWidth = width;
    g.stroke();
  }

  function ring(x, y, r, color) {
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.strokeStyle = color;
    ctx.lineWidth = 3;
    ctx.stroke();
  }

  function drawText(x, y, text, color, font) {
    ctx.fillStyle = color;
    ctx.font = font;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, x, y + 1);
  }

  // Licht von links oben: heller Fleck oben links, zum Rand hin dunkler (wie eine beleuchtete Kuppel/Kugel)
  function shadeGradient(x, y, r, light, dark) {
    const g = ctx.createRadialGradient(x - r * 0.35, y - r * 0.4, r * 0.05, x, y, r * 1.05);
    g.addColorStop(0, 'rgba(255,255,255,' + light + ')');
    g.addColorStop(0.55, 'rgba(255,255,255,0)');
    g.addColorStop(1, 'rgba(0,0,0,' + dark + ')');
    return g;
  }

  function token(x, y, color, label, radius = 0.6) {
    const r = size * radius;
    // Schatten fällt nach rechts unten
    ctx.beginPath();
    ctx.ellipse(x + r * 0.18, y + r * 0.3, r * 0.95, r * 0.8, 0, 0, Math.PI * 2);
    ctx.fillStyle = 'rgba(0,0,0,0.35)';
    ctx.fill();
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.fillStyle = shadeGradient(x, y, r, 0.45, 0.4);
    ctx.fill();
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.stroke();
    drawText(x, y, label, '#ffffff', 'bold ' + Math.round(size * 0.5) + 'px sans-serif');
  }

  function drawTerrain(t) {
    const { x, y } = t;
    hexPath(x, y);
    if (!explored[t.i]) {
      ctx.fillStyle = COL.fog;
      ctx.fill();
      ctx.strokeStyle = '#0a0d10';
      ctx.lineWidth = 1;
      ctx.stroke();
      return;
    }
    let fill;
    if (t.blocked) fill = COL.wall;
    else if (t.forest) fill = COL.forest;
    else fill = ((t.q * 7 + t.r * 3) & 1) ? COL.plain1 : COL.plain2;
    if (inReach(t)) fill = COL.reach;
    ctx.fillStyle = fill;
    ctx.fill();
    ctx.strokeStyle = '#0f1412';
    ctx.lineWidth = 1;
    ctx.stroke();

    // Relief: Licht kommt von links oben. Steigt das Feld zum Licht hin an, ist es hell, sonst dunkel; höhere Felder sind etwas heller.
    if (t.vh && heightK > 0) {
      let gx = 0, gy = 0;
      for (let k = 0; k < 6; k++) { gx += t.vh[k] * HEX_CORNERS[k][0]; gy += t.vh[k] * HEX_CORNERS[k][1]; }
      const lit = (-gx * 0.6 - gy * 0.8) / 3;               // > 0: zum Licht hin ansteigend
      const a = Math.max(-0.5, Math.min(0.5, lit * 0.35)) + t.h * 0.03;
      if (Math.abs(a) > 0.01) fillHex(x, y, a > 0 ? 'rgba(255,255,255,' + a.toFixed(3) + ')' : 'rgba(0,0,0,' + (-a).toFixed(3) + ')');
    }

    if (t.forest) {
      ctx.fillStyle = COL.tree;
      ctx.beginPath();
      ctx.arc(x, y, size * 0.35, 0, Math.PI * 2);
      ctx.fill();
    }
    if (t.blocked) fillHex(x, y, COL.wallTop, size * 0.6);
    if (!visible[t.i]) fillHex(x, y, COL.dim);
  }

  function drawCity(t, state) {
    const { x, y } = t;
    const s = size * 0.42;
    if (state === 'ruin') {
      ctx.strokeStyle = '#e03131';
      ctx.lineWidth = Math.max(2, size * 0.12);
      ctx.beginPath();
      ctx.moveTo(x - s, y - s); ctx.lineTo(x + s, y + s);
      ctx.moveTo(x + s, y - s); ctx.lineTo(x - s, y + s);
      ctx.stroke();
      return;
    }
    ctx.fillStyle = '#e8c15a';
    ctx.fillRect(x - s, y - s, s * 2, s * 2);
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.strokeRect(x - s, y - s, s * 2, s * 2);
  }

  function drawTower(t, owner) {
    const { x, y } = t;
    const s = size * 0.4;
    ctx.beginPath();
    ctx.moveTo(x, y - s);
    ctx.lineTo(x + s * 0.85, y + s * 0.7);
    ctx.lineTo(x - s * 0.85, y + s * 0.7);
    ctx.closePath();
    ctx.fillStyle = owner === 'player' ? COL.owned : COL.neutral;
    ctx.fill();
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.stroke();
  }

  // Städte und Türme: nur wenn je gesehen; im Nebel der letzte bekannte Zustand
  function drawStructures(t) {
    if (!explored[t.i]) return;
    const vis = visible[t.i];
    if (t.city) drawCity(t, vis ? t.city.state : t.seenCity);
    if (t.tower) drawTower(t, vis ? t.tower.owner : t.seenTower);
  }

  function drawArrow(a, b, color) {
    const len = Math.hypot(b.x - a.x, b.y - a.y);
    const dx = (b.x - a.x) / len, dy = (b.y - a.y) / len;
    const hs = size * 0.3;
    const tipX = b.x - dx * size * 0.2, tipY = b.y - dy * size * 0.2;
    const baseX = tipX - dx * hs, baseY = tipY - dy * hs;
    const startX = a.x + dx * size * 0.55, startY = a.y + dy * size * 0.55;
    const px = -dy * hs * 0.6, py = dx * hs * 0.6;

    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    ctx.lineWidth = Math.max(2, size * 0.12);
    ctx.setLineDash([size * 0.2, size * 0.15]);
    ctx.beginPath();
    ctx.moveTo(startX, startY);
    ctx.lineTo(baseX, baseY);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.beginPath();
    ctx.moveTo(tipX, tipY);
    ctx.lineTo(baseX + px, baseY + py);
    ctx.lineTo(baseX - px, baseY - py);
    ctx.closePath();
    ctx.fill();
  }

  function drawIntents() {
    let attacked = false;
    for (const it of intents) {
      let from = here(it.enemy);
      if (!visible[from.i]) continue;
      // Ein Pfeil pro Schritt; ohne Schritte bleibt der Gegner stehen
      for (const s of it.steps) {
        atScale(from, () => drawArrow(from, s.to, s.wander ? COL.wander : COL.intent));
        if (s.destroys) atScale(s.to, () => strokeHex(s.to.x, s.to.y, COL.threat, 3));
        from = s.to;
      }
      if (it.attack) attacked = true;
    }
    // Roter Ring: ein sichtbarer Gegner greift an, falls der Spieler stehen bleibt
    if (attacked) {
      const p = here(player);
      atScale(p, () => ring(p.x, p.y, size * 0.8, COL.threat));
    }
  }

  // Zeichnet das Spielfeld in den Puffer (erst nach layout() und nur mit Fläche)
  function paint() {
    if (!size) return;
    ctx.clearRect(0, 0, canvas.clientWidth, canvas.clientHeight);

    const onScreen = tilesOnScreen();
    // Erst das Bekannte, dann die Beleuchtung, zuletzt der Nebel: Unbekanntes bleibt unbeleuchtet dunkel
    const known = onScreen.filter((t) => explored[t.i]);
    const unknown = onScreen.filter((t) => !explored[t.i]);
    for (const t of known) atScale(t, () => drawTerrain(t));
    for (const t of known) atScale(t, () => drawStructures(t));

    // Sturm (auch im Nebel bekannt), hell: wo er nach diesem Zug sein wird
    const next = nextStormFront();
    const hatchNow = hatchPattern(COL.stormHatch), hatchNext = hatchPattern(COL.stormNextHatch);
    const drawStorm = (t) => {
      const depth = stormDepth(t);
      if (depth <= next) return;
      const now = depth > stormFront;
      atScale(t, () => {
        fillHex(t.x, t.y, now ? COL.storm : COL.stormNext);
        hatchHex(t.x, t.y, now ? hatchNow : hatchNext);
      });
    };
    for (const t of known) drawStorm(t);

    // Ziel
    const g = here(GOAL);
    if (explored[g.i]) atScale(g, () => drawText(g.x, g.y, '★', '#ffd43b', Math.round(size * 1.1) + 'px sans-serif'));

    // Licht und Schatten über das ganze Spielfeld, als läge es auf einer Kuppel: um den Spieler heller,
    // zum Rand hin dunkler. 'source-atop' wirkt nur auf bereits gezeichnete Felder, nicht auf den leeren Hintergrund.
    {
      const p = here(player);
      const R = Math.hypot(canvas.clientWidth, canvas.clientHeight) / 2;
      const g = ctx.createRadialGradient(p.x - R * 0.12, p.y - R * 0.18, 0, p.x, p.y, R);
      g.addColorStop(0, 'rgba(255,255,255,0.28)');
      g.addColorStop(0.3, 'rgba(255,255,255,0.06)');
      g.addColorStop(0.55, 'rgba(0,0,0,0.12)');
      g.addColorStop(1, 'rgba(0,0,0,0.6)');
      ctx.save();
      ctx.globalCompositeOperation = 'source-atop';
      ctx.fillStyle = g;
      ctx.fillRect(0, 0, canvas.clientWidth, canvas.clientHeight);
      ctx.restore();
    }
    for (const t of unknown) atScale(t, () => drawTerrain(t));
    for (const t of unknown) drawStorm(t);

    // Gegner: nur sichtbare
    for (const e of enemies) {
      const t = here(e);
      if (!visible[t.i]) continue;
      const type = ENEMY_TYPES[e.type];
      atScale(t, () => {
        if (!over && dist(e, player) === 1) ring(t.x, t.y, size * (type.radius + 0.15), COL.highlight);
        token(t.x, t.y, type.color, type.letter + e.hp, type.radius);
      });
    }

    // Letzte bekannte Position des Spielers aus Gegnersicht, markiert mit "?"
    for (const e of enemies) {
      if (!e.lastSeen || !visible[here(e).i]) continue;
      { const ls = here(e.lastSeen); atScale(ls, () => drawText(ls.x, ls.y, '?', COL.neutral, 'bold ' + Math.round(size * 0.6) + 'px sans-serif')); }
    }

    drawIntents();

    const p = here(player);
    atScale(p, () => token(p.x, p.y, COL.player, 'P' + player.hp));
  }

  // Puffer auf den Bildschirm, darüber der Hover-Rahmen
  function present() {
    if (!size) return; // drawImage wirft bei einem Puffer ohne Fläche
    const w = canvas.clientWidth, h = canvas.clientHeight;
    view.clearRect(0, 0, w, h);
    view.drawImage(buffer, 0, 0, w, h);
    const t = hover && here(hover);
    if (t) atScale(t, () => strokeHex(t.x, t.y, COL.highlight, 2, view));
  }

  // ---------- Spieler ----------
  // Breitensuche: welche Felder erreicht der Spieler mit seiner Laufweite?
  // Vom Aussichtsturm aus zusätzlich: Sprung über ein angrenzendes Mauerfeld (zählt als Laufen)
  function computeReach() {
    if (over) { reach = null; return; }
    reach = distanceField([player], isFree, PLAYER_MOVES);
    const p = here(player);
    if (!p.tower) return;
    for (const land of jumpTargets(p)) {
      if (reach[land.i] === -1 && isFree(land)) reach[land.i] = 2;
    }
  }

  // Eine Aktion pro Zug (laufen oder angreifen); danach sind sofort die Gegner dran
  function handleAction(h) {
    const t = here(h);
    if (!t) return;

    // Angriff auf benachbarten Gegner, mit voller Kraft: Schaden = eigene HP
    const enemy = enemyAt(t);
    if (enemy) {
      if (dist(enemy, player) === 1) {
        const name = ENEMY_TYPES[enemy.type].name;
        const rageBefore = rage;
        if (damageEnemy(enemy, player.hp, true)) {
          log('Du besiegst einen ' + name + '. Alarm -' + (rageBefore - rage) + '.');
        } else {
          log('Du triffst einen ' + name + ' (HP ' + enemy.hp + ').');
        }
        endTurn();
      } else {
        setMsg('Zu weit weg: Angreifen geht nur auf ein Nachbarfeld.');
      }
      return;
    }

    // Bewegung (bis zu PLAYER_MOVES Felder)
    if (inReach(t)) {
      player.q = t.q;
      player.r = t.r;
      if (isIntact(t)) captureCity(t);
      if (t.tower && t.tower.owner === null) {
        t.tower.owner = 'player';
        log('Sensorturm besetzt: Sicht über Dschungel und Barrieren. Von einem Sensorturm aus kannst du mit dem Sprungmodul über Barrieren springen.');
      }
      if (samePos(t, GOAL)) {
        over = 'win';
        log('Du hast den Extraktionspunkt erreicht!');
      }
      endTurn();
    }
  }

  // Eingenommene Stadt: gibt einmal HP und verschwindet dann,
  // damit dort kein Räuber mehr einen Kriegsherrn erzeugen kann
  function captureCity(t) {
    t.city = null;
    cityTiles = cityTiles.filter((c) => c !== t);
    captured += 1;
    player.hp += RECRUIT_HP;
    log('Kolonie gesichert: +' + RECRUIT_HP + ' HP.');
  }

  // Am Ende des Gegnerzugs: Sturm rückt vor, wer drinsteht, verliert HP
  function advanceStorm() {
    stormFront -= stormStep(turn);
    for (const e of enemies.filter((x) => inStorm(here(x)))) {
      if (damageEnemy(e, STORM_DAMAGE, false) && visible[here(e).i]) {
        log('Ein ' + ENEMY_TYPES[e.type].name + ' wird vom Ionensturm zerstört.');
      }
    }
    if (inStorm(here(player))) {
      damagePlayer(STORM_DAMAGE, 'Der Ionensturm trifft dich: -' + STORM_DAMAGE + ' HP.', 'Der Ionensturm hat dich erwischt.');
    }
  }

  // ---------- Zugablauf ----------
  // Gegnerzug (genau der angezeigte Plan), Sturm, Wut; dann neu zeichnen.
  // Nach Sieg oder Niederlage wird nur noch neu gezeichnet.
  function endTurn() {
    if (!over) executePlan(intents);
    if (!over) advanceStorm();
    if (!over) {
      turn += 1;
      updateRage();
      log('— Zug ' + turn + ' —');
      setMsg('Dein Zug: bis zu ' + PLAYER_MOVES + ' Felder laufen oder einen Nachbarn angreifen.');
    }
    render();
  }

  // ---------- UI ----------
  function setMsg(t) {
    ui.msg.textContent = t;
  }

  function log(t) {
    const d = document.createElement('div');
    d.textContent = t;
    ui.log.prepend(d);
    while (ui.log.children.length > LOG_LINES) ui.log.lastElementChild.remove();
  }

  function updateHud() {
    ui.turn.textContent = turn;
    ui.hp.textContent = player.hp;
    ui.cities.textContent = captured;
    ui.rage.textContent = rage + ' (Stufe ' + rageLevel() + ')';
    ui.rageBar.value = rage;
    ui.endBtn.disabled = !!over;

    if (over === 'win') setMsg('Sieg! Du hast den Extraktionspunkt erreicht.');
    else if (over === 'lose') setMsg('Niederlage. Drück auf "Neu" für einen neuen Versuch.');
  }

  function show() {
    placeTiles(); // Kamera folgt dem Spieler
    paint();
    present();
    updateHud();
  }

  // Abgeleiteten Zustand neu berechnen (Sicht, Laufweite, Gegnerplan), dann zeichnen
  function render() {
    updateVision();
    computeReach();
    intents = computeIntents();
    show();
    if (over) reportEnd(over);
  }

  // ---------- Spielstart ----------
  // Reihe -> Gebiet (0 = Norden) oder -1 für eine Trennmauer
  const areaOfRow = (row) =>
    row % (AREA_ROWS + 1) === AREA_ROWS ? -1 : Math.floor(row / (AREA_ROWS + 1));
  // Nicht direkt an einer Trennmauer: dort keine zufälligen Mauern, damit die Durchgänge frei bleiben
  const awayFromSeparator = (t) => {
    const inArea = t.row % (AREA_ROWS + 1);
    return inArea >= 1 && inArea <= AREA_ROWS - 2;
  };

  // Startspalten von n Durchgängen, mit Abstand zueinander
  function gapStarts(n) {
    for (let tries = 0; tries < 200; tries++) {
      const s = Array.from({ length: n }, () => rand(0, MAP_WIDTH - GAP_WIDTH)).sort((a, b) => a - b);
      if (s.every((x, i) => i === 0 || x - s[i - 1] >= GAP_WIDTH + 2)) return s;
    }
    // Passt es zufällig nicht: gleichmäßig verteilen
    return Array.from({ length: n }, (_, i) => Math.round((i + 0.5) * MAP_WIDTH / n - GAP_WIDTH / 2));
  }

  // Zufällige Hügel: runde Kuppen mit weichem Rand, je Gebiet. Rund um den Start bleibt es flach.
  function buildHeights() {
    for (const t of tiles) t.h = 0;
    for (let a = 0; a < AREA_COUNT; a++) {
      const inArea = tiles.filter((t) => t.area === a);
      const hills = [];
      for (let k = rand(HILLS[0], HILLS[1]); k > 0; k--) {
        hills.push({ at: pick(inArea), r: HILL_RADIUS[0] + Math.random() * (HILL_RADIUS[1] - HILL_RADIUS[0]), h: 1 + Math.random() * HEIGHT_MAX });
      }
      for (const t of inArea) {
        let v = 0;
        for (const hl of hills) {
          const d = dist(t, hl.at);
          if (d < hl.r) v = Math.max(v, hl.h * (1 - d / hl.r));
        }
        t.h = dist(t, START) <= FLAT_AT_START ? 0 : Math.max(0, Math.min(HEIGHT_MAX, Math.round(v)));
      }
    }
    // Eckenhöhe = Mittel der Felder, die sich die Ecke teilen; so bleiben die Kanten benachbarter Felder zusammen
    const acc = new Map();
    const keyOf = (t, k) => {
      const cx = 1.5 * t.q, cy = SQRT3 * (t.r + t.q / 2);
      return Math.round((cx + HEX_CORNERS[k][0]) * 20) + ',' + Math.round((cy + HEX_CORNERS[k][1]) * 20);
    };
    for (const t of tiles) {
      t.vk = HEX_CORNERS.map((_, k) => keyOf(t, k));
      for (const key of t.vk) {
        const e = acc.get(key) || { sum: 0, n: 0 };
        e.sum += t.h; e.n++;
        acc.set(key, e);
      }
    }
    for (const t of tiles) t.vh = t.vk.map((key) => { const e = acc.get(key); return e.sum / e.n; });
  }

  // Gelände: Felder, Trennmauern, zufällige Mauern und Wälder je Gebiet
  function buildTerrain() {
    tiles = [];
    grid = new Array(MAP_ROWS * MAP_WIDTH);
    for (let row = 0; row < MAP_ROWS; row++) {
      for (let col = 0; col < MAP_WIDTH; col++) {
        const { q, r } = toAxial(col, row);
        const t = {
          i: tiles.length, q, r, row, area: areaOfRow(row), x: 0, y: 0, nb: [],
          blocked: false, forest: false, h: 0,
          city: null, tower: null,
          seenCity: null, seenTower: null
        };
        tiles.push(t);
        grid[row * MAP_WIDTH + col] = t;
      }
    }
    for (const t of tiles) {
      t.nb = DIRS.map(([dq, dr]) => tileAt(t.q + dq, t.r + dr)).filter(Boolean);
    }

    buildHeights();

    // Trennmauern: eine ganze Reihe Mauer bis auf die Durchgänge
    SEPARATOR_GAPS.forEach((gaps, k) => {
      const row = (k + 1) * (AREA_ROWS + 1) - 1;
      const starts = gapStarts(gaps);
      for (let col = 0; col < MAP_WIDTH; col++) {
        grid[row * MAP_WIDTH + col].blocked = !starts.some((s) => col >= s && col < s + GAP_WIDTH);
      }
    });

    for (let a = 0; a < AREA_COUNT; a++) {
      // Mauern: kurze zufällige Ketten (unpassierbar, blockieren Sicht)
      const wallOk = (p) => p.area === a && awayFromSeparator(p) && farFromEnds(p);
      for (let s = 0; s < WALL_SEGMENTS; s++) {
        let t = randomFreeTile(wallOk);
        const len = rand(WALL_LEN[0], WALL_LEN[1]);
        for (let i = 0; i < len && t; i++) {
          t.blocked = true;
          const opts = t.nb.filter((n) => wallOk(n) && !n.blocked);
          t = opts.length ? pick(opts) : null;
        }
      }

      // Wälder: zusammenhängende Flächen (passierbar, blockieren Sicht)
      const forestOk = (p) => p.area === a && farFromEnds(p);
      for (let b = 0; b < FOREST_BLOBS; b++) {
        const seed = randomFreeTile(forestOk);
        if (!seed) continue;
        const want = rand(FOREST_SIZE[0], FOREST_SIZE[1]);
        const cells = [seed];
        seed.forest = true;
        for (let guard = 0; cells.length < want && guard < FOREST_TRIES; guard++) {
          const opts = pick(cells).nb.filter((n) => forestOk(n) && !n.blocked && !n.forest);
          if (!opts.length) continue;
          const n = pick(opts);
          n.forest = true;
          cells.push(n);
        }
      }
    }
  }

  function init() {
    over = false;
    captured = 0;
    rage = 0;
    spawnCounter = 0;
    nextArmyId = 0;
    stormFront = MAX_STORM_DEPTH; // tiefster Punkt der Karte: noch kein Sturm
    turn = 1;
    hover = null;
    ui.log.replaceChildren();

    player = { q: START.q, r: START.r, hp: PLAYER_START_HP };
    enemies = [];

    // Neu würfeln, bis das Ziel vom Start aus erreichbar ist
    const winnable = () => distanceField([START])[here(GOAL).i] !== -1;
    for (let tries = 0; tries < TERRAIN_TRIES; tries++) {
      buildTerrain();
      if (winnable()) break;
    }
    // Notlösung: zufällige Mauern entfernen. Übrig bleiben die Trennmauern mit ihren Durchgängen,
    // und vor denen liegen nie zufällige Mauern; damit gibt es immer einen Weg.
    if (!winnable()) {
      for (const t of tiles) if (t.area !== -1) t.blocked = false;
    }
    explored = new Uint8Array(tiles.length);
    sightCache = new Map();

    // Verteilt count Felder, die filter erfüllen, mit Mindestabstand untereinander
    // Mit Wahrscheinlichkeit preferChance wird zuerst ein Feld gesucht, das zusätzlich prefer erfüllt.
    const placeSpread = (count, filter, prefer = null, preferChance = 0) => {
      const out = [];
      for (let i = 0; i < count; i++) {
        const ok = (p) => filter(p) && out.every((x) => dist(x, p) >= MIN_SPACING);
        const t = (prefer && Math.random() < preferChance && randomFreeTile((p) => ok(p) && prefer(p)))
          || randomFreeTile(ok);
        if (!t) break;
        out.push(t);
      }
      return out;
    };

    cityTiles = [];
    towerTiles = [];
    for (let a = 0; a < AREA_COUNT; a++) {
      // Städte: neutral, nicht im Wald
      const cities = placeSpread(CITY_COUNT, (t) => t.area === a && !t.forest && dist(t, START) >= 4 && dist(t, GOAL) >= 3);
      for (const t of cities) t.city = { state: 'neutral' };
      cityTiles.push(...cities);

      // Aussichtstürme: neutral; Sichtfeld ist fest, weil Hindernisse egal sind
      // bevorzugt an einer Mauer, über die man springen kann
      const towers = placeSpread(TOWER_COUNT, (t) => t.area === a && !t.city && farFromEnds(t),
        (t) => jumpTargets(t).length > 0, TOWER_AT_WALL);
      for (const t of towers) {
        t.tower = { owner: null, sight: tiles.filter((x) => dist(t, x) <= TOWER_RANGE).map((x) => x.i) };
      }
      towerTiles.push(...towers);

      // Start-Gegner
      const startOk = (p) => p.area === a && dist(p, START) >= 4 && dist(p, GOAL) >= 2 && !p.city;
      for (const [typeId, cfg] of Object.entries(ENEMY_TYPES)) {
        for (let i = 0; i < cfg.count; i++) placeEnemy(typeId, startOk);
      }
    }

    log('Gelb = Kolonie, Turm = Sensor. Stern = Extraktion. Pfeil = nächster Zug eines Gegners. Nebel = unbekannt.');
    setMsg('Dein Zug: Tippe auf ein grünes Feld zum Laufen oder auf einen Nachbarn zum Angreifen.');
    render();
  }

  // ---------- Events ----------
  const eventHex = (e) => {
    const rect = canvas.getBoundingClientRect();
    return toHex(e.clientX - rect.left, e.clientY - rect.top);
  };

  canvas.addEventListener('click', (e) => {
    if (over) return;
    handleAction(eventHex(e));
  });

  canvas.addEventListener('pointermove', (e) => {
    if (e.pointerType !== 'mouse') return; // Touch hat keine Hover-Anzeige
    const h = eventHex(e);
    if (!hover || !samePos(hover, h)) {
      hover = h;
      present();
    }
  });

  canvas.addEventListener('pointerleave', () => {
    hover = null;
    present();
  });

  ui.endBtn.addEventListener('click', endTurn);
  el('restartBtn').addEventListener('click', init);

  // ---------- Aquarell-Filter (Testversion) ----------
  // Nur Darstellung: SVG-Filter auf dem Spielfeld (wackelige Kanten, Weichzeichnung, Pigmentrand) plus
  // Papierkörnung und Farbflecken als Überlagerung. Die Regler oben rechts stellen alles live ein.
  {
    const WC_DEFAULT = { on: true, edge: 5, size: 0.03, soft: 0.5, rim: 0.3, grain: 0.55, wash: 0.55, sat: 0.88, bright: 1.1, contrast: 0.94, hue: 0, hk: 0.07 };
    const PRESETS = {
      Zart:   { edge: 3, size: 0.03, soft: 0.4, rim: 0.2, grain: 0.35, wash: 0.3, sat: 0.95, bright: 1.05, contrast: 0.97, hue: 0 },
      Mittel: { edge: 5, size: 0.03, soft: 0.5, rim: 0.3, grain: 0.55, wash: 0.55, sat: 0.88, bright: 1.1, contrast: 0.94, hue: 0 },
      Stark:  { edge: 11, size: 0.02, soft: 1.1, rim: 0.6, grain: 0.85, wash: 0.9, sat: 0.7, bright: 1.18, contrast: 0.88, hue: 0 }
    };
    const KEY = 'travellershex-aquarell';
    const wc = { ...WC_DEFAULT };
    try { Object.assign(wc, JSON.parse(localStorage.getItem(KEY) || '{}')); } catch (e) { /* kein gespeicherter Stand */ }

    const svgNS = 'http://www.w3.org/2000/svg';
    const defs = document.createElementNS(svgNS, 'svg');
    defs.setAttribute('width', '0');
    defs.setAttribute('height', '0');
    defs.style.position = 'absolute';
    defs.innerHTML =
      '<filter id="watercolor" x="-2%" y="-2%" width="104%" height="104%" color-interpolation-filters="sRGB">' +
      '<feTurbulence id="wcTurb" type="fractalNoise" baseFrequency="0.03" numOctaves="2" seed="7" result="warp"/>' +
      '<feDisplacementMap id="wcDisp" in="SourceGraphic" in2="warp" scale="5" xChannelSelector="R" yChannelSelector="G" result="wobble"/>' +
      '<feGaussianBlur id="wcSoft" in="wobble" stdDeviation="0.5" result="soft"/>' +
      '<feGaussianBlur in="soft" stdDeviation="3.5" result="pool"/>' +
      // Pigment sammelt sich an Kanten: weiche Unscharf-Maske ((1 + rim) * scharf - rim * unscharf)
      '<feComposite id="wcRim" in="soft" in2="pool" operator="arithmetic" k1="0" k2="1.3" k3="-0.3"/>' +
      '</filter>';
    document.body.appendChild(defs);

    const noise = (freq, octaves, size, rgb, alphaGain, alphaBias) => "url(\"data:image/svg+xml;utf8," +
      encodeURIComponent("<svg xmlns='http://www.w3.org/2000/svg' width='" + size + "' height='" + size + "'><filter id='f'>" +
        "<feTurbulence type='fractalNoise' baseFrequency='" + freq + "' numOctaves='" + octaves + "' stitchTiles='stitch'/>" +
        "<feColorMatrix values='0 0 0 0 " + rgb[0] + "  0 0 0 0 " + rgb[1] + "  0 0 0 0 " + rgb[2] + "  0 0 0 " + alphaGain + " " + alphaBias + "'/>" +
        "</filter><rect width='100%' height='100%' filter='url(#f)'/></svg>") + "\")";

    const style = document.createElement('style');
    style.textContent =
      '.paper { position: fixed; inset: 0; pointer-events: none; display: none; mix-blend-mode: multiply; z-index: 4; }' +
      '#paperGrain { background-image: ' + noise(0.9, 2, 300, [0.45, 0.4, 0.33], 1.1, -0.35) + '; }' +
      '#paperWash { background-image: ' + noise(0.007, 3, 900, [0.55, 0.62, 0.7], 1.6, -0.45) + '; }' +
      'body.aquarell .paper { display: block; }' +
      '#wcBox { position: fixed; right: 8px; top: 56px; z-index: 6; width: 250px; max-height: calc(100vh - 140px); overflow-y: auto; background: rgba(10,16,13,0.9); border-radius: 8px; padding: 6px 10px; font-size: 13px; color: #e8efe9; }' +
      '#wcBox summary { cursor: pointer; padding: 2px 0; }' +
      '#wcBox label { display: block; margin-top: 8px; }' +
      '#wcBox input[type=range] { width: 100%; }' +
      '#wcBox .row { display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap; align-items: center; }' +
      '#wcBox button { padding: 4px 10px; font-size: 13px; }';
    document.head.appendChild(style);

    for (const id of ['paperGrain', 'paperWash']) {
      const d = document.createElement('div');
      d.id = id;
      d.className = 'paper';
      document.body.appendChild(d);
    }

    const SLIDERS = [
      ['edge', 'Kanten wackeln', 0, 20, 0.5],
      ['size', 'Wackel-Frequenz (höher = feiner)', 0.008, 0.08, 0.002],
      ['soft', 'Weichzeichnung', 0, 2, 0.1],
      ['rim', 'Pigmentrand', 0, 1, 0.05],
      ['grain', 'Papierkörnung', 0, 1, 0.05],
      ['wash', 'Farbflecken', 0, 1, 0.05],
      ['sat', 'Sättigung', 0.3, 1.4, 0.02],
      ['bright', 'Helligkeit', 0.7, 1.4, 0.02],
      ['contrast', 'Kontrast', 0.7, 1.3, 0.02],
      ['hue', 'Farbton verschieben (°)', -60, 60, 2],
      ['hk', 'Höhen-Effekt (0 = flach)', 0, 0.15, 0.01]
    ];
    const box = document.createElement('details');
    box.id = 'wcBox';
    box.innerHTML = '<summary>Einstellungen (Aquarell, Höhe)</summary>' +
      '<div class="row"><input id="wcOn" type="checkbox"><label for="wcOn" style="margin:0">Filter an</label></div>' +
      '<div class="row" id="wcPresets"></div>' +
      SLIDERS.map(([k, name, lo, hi, step]) =>
        '<label>' + name + ': <b id="wcv_' + k + '"></b><input id="wc_' + k + '" type="range" min="' + lo + '" max="' + hi + '" step="' + step + '"></label>').join('') +
      '<div class="row"><button id="wcReset">Zurücksetzen</button></div>';
    document.body.appendChild(box);
    const $$ = (id) => box.querySelector('#' + id);
    for (const name of Object.keys(PRESETS)) {
      const b = document.createElement('button');
      b.textContent = name;
      b.addEventListener('click', () => { Object.assign(wc, PRESETS[name]); apply(); });
      $$('wcPresets').appendChild(b);
    }

    function apply() {
      document.body.classList.toggle('aquarell', wc.on);
      document.getElementById('wcDisp').setAttribute('scale', wc.edge);
      document.getElementById('wcTurb').setAttribute('baseFrequency', wc.size);
      document.getElementById('wcSoft').setAttribute('stdDeviation', wc.soft);
      const rim = document.getElementById('wcRim');
      rim.setAttribute('k2', 1 + wc.rim);
      rim.setAttribute('k3', -wc.rim);
      if (heightK !== wc.hk) {
        heightK = wc.hk;
        if (typeof player !== 'undefined' && player && size) { placeTiles(); render(); }
      }
      document.getElementById('paperGrain').style.opacity = wc.grain;
      document.getElementById('paperWash').style.opacity = wc.wash;
      canvas.style.filter = wc.on
        ? 'url(#watercolor) saturate(' + wc.sat + ') brightness(' + wc.bright + ') contrast(' + wc.contrast + ') hue-rotate(' + wc.hue + 'deg)'
        : '';
      $$('wcOn').checked = wc.on;
      for (const [k] of SLIDERS) {
        $$('wc_' + k).value = wc[k];
        $$('wcv_' + k).textContent = Number(wc[k]).toFixed(k === 'size' ? 3 : 2).replace(/\.?0+$/, '');
      }
      try { localStorage.setItem(KEY, JSON.stringify(wc)); } catch (e) { /* Speicher nicht verfügbar */ }
    }
    for (const [k] of SLIDERS) {
      $$('wc_' + k).addEventListener('input', (e) => { wc[k] = Number(e.target.value); apply(); });
    }
    $$('wcOn').addEventListener('change', (e) => { wc.on = e.target.checked; apply(); });
    $$('wcReset').addEventListener('click', () => { Object.assign(wc, WC_DEFAULT); apply(); });
    apply();
  }

  // ---------- Einbindung ----------
  // Gestartet wird von außen (index.js): HexGame.start({ onEnd }).
  // Das Ergebnis geht einmal an onEnd: 'win', 'lose' oder 'abort' (Abbrechen-Knopf, nur in der Weltkarte).
  // Ohne onEnd (battle.html) läuft der Kampf einfach für sich.
  let onEnd = null;
  function reportEnd(result) {
    if (!onEnd) return;
    const cb = onEnd;
    onEnd = null;
    cb(result);
  }
  window.HexGame = {
    start(opts = {}) {
      onEnd = opts.onEnd || null;
      init();
    }
  };
  el('exitBtn')?.addEventListener('click', () => reportEnd('abort'));
  // Beobachtet auch Fenstergrößen-Änderungen; der erste Aufruf setzt die Canvas-Größe
  new ResizeObserver(layout).observe(canvas);
  // Pixeldichte-Wechsel (z. B. anderer Monitor) ändert die Größe nicht, braucht aber einen neuen Puffer
  const watchDpr = () => {
    matchMedia('(resolution: ' + window.devicePixelRatio + 'dppx)')
      .addEventListener('change', () => { layout(); watchDpr(); }, { once: true });
  };
  watchDpr();
})();
