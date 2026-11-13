(() => {
  'use strict';

  // ---------- Konfiguration ----------
  const MAP_BG = '#1d1710';             // Rand um das Bild, wenn das Fenster ein anderes Seitenverhältnis hat
  const START_ID = 'landezone';
  const SAVE_KEY = 'travellers-welt';
  const POI_RADIUS = 14;        // Radius eines Ortes in Pixeln bei voller Größe (Bild ab 1400 px breit)
  const BATTLE_CLOSE_DELAY = 1500; // ms, damit man das Kampfergebnis noch sieht

  // Orte: Position als Anteil der Bildbreite/-höhe (0..1), passend zu den Gebäuden auf worldmap.png.
  // links = Routen, folgen grob den Straßen im Bild (immer symmetrisch).
  const PLACES = [
    { id: 'landezone', name: 'Landezone Alpha',   x: 0.328, y: 0.711, links: ['relais'] },
    { id: 'relais',    name: 'Relais Sigma',      x: 0.400, y: 0.658, links: ['landezone', 'aurum', 'ferrum', 'kepler'] },
    { id: 'aurum',     name: 'Zitadelle Aurum',   x: 0.345, y: 0.533, links: ['relais', 'myzel', 'elysion'] },
    { id: 'myzel',     name: 'Myzel-Außenposten', x: 0.372, y: 0.204, links: ['aurum'] },
    { id: 'kepler',    name: 'Siedlung Kepler',   x: 0.510, y: 0.729, links: ['relais', 'ferrum', 'kristall'] },
    { id: 'ferrum',    name: 'Ferrum-Werke',      x: 0.405, y: 0.880, links: ['relais', 'kepler', 'halo'] },
    { id: 'halo',      name: 'Ringstadt Halo',    x: 0.540, y: 0.889, links: ['ferrum', 'kristall'] },
    { id: 'kristall',  name: 'Kristallspitzen',   x: 0.645, y: 0.764, links: ['kepler', 'halo', 'vega', 'nimbus'] },
    { id: 'vega',      name: 'Glaskuppel Vega',   x: 0.765, y: 0.827, links: ['kristall'] },
    { id: 'nimbus',    name: 'Wolkenturm Nimbus', x: 0.783, y: 0.373, links: ['kristall', 'elysion'] },
    { id: 'elysion',   name: 'Kuppelstadt Elysion', x: 0.760, y: 0.222, links: ['aurum', 'nimbus'] }
  ];

  // ---------- DOM ----------
  const canvas = document.getElementById('map');
  const ctx = canvas.getContext('2d');
  const mapMsg = document.getElementById('mapMsg');
  const battleEl = document.getElementById('battle');

  // ---------- Zustand ----------
  let unlocked = new Set([START_ID]);
  let current = START_ID;
  let battleOpen = false;
  let pending = null;           // Ort, um den gerade gekämpft wird
  let hoverId = null;
  let W = 0, H = 0;
  let OX = 0, OY = 0, IW = 1, IH = 1; // Lage des Bildes auf dem Canvas
  let poiR = POI_RADIUS, fontPx = 15; // schrumpfen mit kleinen Fenstern, damit Namen nicht überlappen
  let img = null, imgReady = false;

  // ---------- Hilfsfunktionen ----------
  const byId = (id) => PLACES.find((p) => p.id === id);
  const adjacent = (a, b) => {
    const pa = byId(a), pb = byId(b);
    return !!pa && !!pb && (pa.links.includes(b) || pb.links.includes(a));
  };
  const toScr = (x, y) => ({ x: OX + x * IW, y: OY + y * IH });
  const hitTest = (px, py) => {
    for (const p of PLACES) {
      const s = toScr(p.x, p.y);
      if (Math.hypot(px - s.x, py - s.y) <= poiR + 4) return p;
    }
    return null;
  };

  // ---------- Speichern ----------
  function save() {
    try {
      localStorage.setItem(SAVE_KEY, JSON.stringify({ unlocked: [...unlocked], current }));
    } catch (e) { /* Speicher nicht verfügbar: egal */ }
  }

  function load() {
    try {
      const d = JSON.parse(localStorage.getItem(SAVE_KEY));
      if (d && Array.isArray(d.unlocked) && byId(d.current)) {
        unlocked = new Set(d.unlocked);
        current = d.current;
      }
    } catch (e) { /* kein gültiger Stand */ }
  }

  // ---------- Zeichnen ----------
  // Routen: alle dünn, die vom aktuellen Ort aus kräftig (das Bild hat schon eigene Straßen)
  function drawRoads() {
    ctx.setLineDash([8, 6]);
    const drawn = new Set();
    for (const p of PLACES) {
      for (const l of p.links) {
        const k = [p.id, l].sort().join('|');
        if (drawn.has(k)) continue;
        drawn.add(k);
        const active = p.id === current || l === current;
        const a = toScr(p.x, p.y);
        const b = toScr(byId(l).x, byId(l).y);
        ctx.strokeStyle = active ? 'rgba(28,126,214,0.9)' : 'rgba(60,40,20,0.45)';
        ctx.lineWidth = active ? 4 : 2;
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.stroke();
      }
    }
    ctx.setLineDash([]);
  }

  function drawPlaces() {
    for (const p of PLACES) {
      const { x, y } = toScr(p.x, p.y);
      const isFree = unlocked.has(p.id);
      const isHere = p.id === current;
      const challenge = !isFree && adjacent(current, p.id);

      ctx.beginPath();
      ctx.arc(x, y, poiR, 0, Math.PI * 2);
      ctx.fillStyle = isFree ? '#5c8a4a' : '#7a7468';
      ctx.fill();
      ctx.lineWidth = 3;
      ctx.strokeStyle = challenge ? '#ffd43b' : '#2b2118';
      ctx.stroke();

      if (isHere) {
        ctx.beginPath();
        ctx.arc(x, y, poiR + 5, 0, Math.PI * 2);
        ctx.strokeStyle = '#1c7ed6';
        ctx.lineWidth = 3;
        ctx.stroke();
      }

      ctx.font = 'bold ' + fontPx + 'px Georgia, serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      ctx.lineWidth = 4;
      ctx.strokeStyle = 'rgba(255,250,235,0.9)';
      ctx.strokeText(p.name, x, y + poiR + 5);
      ctx.fillStyle = '#2b2118';
      ctx.fillText(p.name, x, y + poiR + 5);
    }
  }

  function draw() {
    ctx.fillStyle = MAP_BG;
    ctx.fillRect(0, 0, W, H);
    if (imgReady) ctx.drawImage(img, OX, OY, IW, IH);
    drawRoads();
    drawPlaces();
  }

  // Bild so groß wie möglich einpassen, Seitenverhältnis bleibt erhalten
  function fitImage() {
    const ratio = imgReady ? img.naturalWidth / img.naturalHeight : 16 / 9;
    IW = Math.min(W, H * ratio);
    IH = IW / ratio;
    OX = (W - IW) / 2;
    OY = (H - IH) / 2;
    const k = Math.max(0.6, Math.min(1, IW / 1400));
    poiR = POI_RADIUS * k;
    fontPx = Math.round(15 * k);
  }

  function layout() {
    const dpr = window.devicePixelRatio || 1;
    W = canvas.clientWidth;
    H = canvas.clientHeight;
    canvas.width = W * dpr;
    canvas.height = H * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    fitImage();
    draw();
  }

  // ---------- Texte ----------
  function setMsg(t) {
    mapMsg.textContent = t;
  }

  function statusText() {
    return 'Du bist in ' + byId(current).name + '. Klicke einen angrenzenden Ort.';
  }

  function describe(p) {
    if (p.id === current) return p.name + ': du bist hier.';
    if (!adjacent(current, p.id)) return p.name + ': keine Route von hier.';
    if (unlocked.has(p.id)) return p.name + ': frei, anklicken zum Reisen.';
    return p.name + ': anklicken für den Einsatz um diesen Ort.';
  }

  // ---------- Kampf ----------
  function startBattle(p) {
    if (!window.HexGame) {
      setMsg('Spielmodul fehlt (game.js).');
      return;
    }
    pending = p;
    battleOpen = true;
    battleEl.hidden = false;
    window.HexGame.start({ onEnd: endBattle });
  }

  // Wird von game.js aufgerufen: 'win', 'lose' oder 'abort'
  function endBattle(result) {
    if (!battleOpen) return;
    battleOpen = false;
    const p = pending;
    pending = null;

    if (result === 'win') {
      unlocked.add(p.id);
      current = p.id;
      save();
      setMsg(p.name + ' gesichert. Neue Routen öffnen sich.');
    } else if (result === 'lose') {
      setMsg(p.name + ' hat dich zurückgeschlagen.');
    } else {
      setMsg('Einsatz abgebrochen.');
    }

    draw();
    setTimeout(() => {
      if (!battleOpen) battleEl.hidden = true;
    }, BATTLE_CLOSE_DELAY);
  }

  // ---------- Events ----------
  canvas.addEventListener('click', (e) => {
    if (battleOpen) return;
    const rect = canvas.getBoundingClientRect();
    const p = hitTest(e.clientX - rect.left, e.clientY - rect.top);
    if (!p) return;

    if (p.id === current || !adjacent(current, p.id)) {
      setMsg(describe(p));
      return;
    }
    if (unlocked.has(p.id)) {
      current = p.id;
      save();
      setMsg('Du fliegst nach ' + p.name + '.');
      draw();
      return;
    }
    startBattle(p);
  });

  canvas.addEventListener('pointermove', (e) => {
    if (e.pointerType !== 'mouse' || battleOpen) return;
    const rect = canvas.getBoundingClientRect();
    const p = hitTest(e.clientX - rect.left, e.clientY - rect.top);
    const id = p ? p.id : null;
    if (id === hoverId) return;
    hoverId = id;
    setMsg(p ? describe(p) : statusText());
  });

  canvas.addEventListener('pointerleave', () => {
    hoverId = null;
    if (!battleOpen) setMsg(statusText());
  });

  // ---------- Start ----------
  // Gestartet wird von außen (index.js): WorldMap.start()
  window.WorldMap = {
    start() {
      load();
      // <img id="mapImg"> in index.html: worldmap.jpg, verkleinerte Kopie von worldmap.png (16 MB wären zu schwer)
      img = document.getElementById('mapImg');
      const ready = () => { imgReady = true; fitImage(); draw(); };
      if (img.complete && img.naturalWidth) ready(); else img.addEventListener('load', ready);
      setMsg(statusText());
      window.addEventListener('resize', layout);
      if (window.ResizeObserver) new ResizeObserver(layout).observe(canvas);
      layout();
    }
  };
})();
