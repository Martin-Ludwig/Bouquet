// Package content liefert die mitgelieferten Spielinhalte als Dateisystem.
// Jeder Unterordner ist ein Mod; "base" ist das Grundspiel. Gelesen werden
// die Dateien von bouquet/mods.
package content

import "embed"

// FS enthält alle mitgelieferten Mods. Beim Bauen werden die Dateien in das
// Programm eingebettet.
//
//go:embed all:base
var FS embed.FS
