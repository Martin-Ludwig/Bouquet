// Einstiegspunkt: wird als letztes Skript geladen und startet die passende Ansicht.
//  index.html (mit #map):  Weltkarte, Kämpfe laufen im Overlay (world.js + game.js)
//  battle.html (ohne):     ein einzelner Kampf zum Testen (game.js)
(() => {
  'use strict';

  if (document.getElementById('map')) {
    window.WorldMap.start();
  } else {
    window.HexGame.start();
  }
})();
