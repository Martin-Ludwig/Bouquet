package main

import "syscall/js"

// Der Spielstand liegt im localStorage des Browsers unter diesem Schlüssel.
const saveKey = "bouquet-save"

// readSave liefert den gespeicherten Spielstand, oder nil, wenn es keinen
// gibt oder der Browser keinen Speicher erlaubt.
func readSave() (data []byte) {
	defer func() {
		if recover() != nil {
			data = nil
		}
	}()
	v := js.Global().Get("localStorage").Call("getItem", saveKey)
	if v.IsNull() || v.IsUndefined() {
		return nil
	}
	return []byte(v.String())
}

// writeSave speichert den Spielstand. Ohne Speicher im Browser passiert
// nichts; das Spiel läuft dann ohne Spielstand weiter.
func writeSave(data []byte) {
	defer func() { _ = recover() }()
	js.Global().Get("localStorage").Call("setItem", saveKey, string(data))
}
