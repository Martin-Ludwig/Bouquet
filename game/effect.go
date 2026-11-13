package game

import "slices"

// Effect ist eine Wirkung, die ein Objekt auslöst, wenn der Spieler es
// betritt. call trägt die Regler aus den Inhalten.
type Effect func(s *State, o *Object, call *EffectCall)

var effects = map[string]Effect{}

// RegisterEffect macht eine Wirkung unter name für Objekte verfügbar
// ("effect" in objects.json). Ein vorhandener Name wird ersetzt. Aufrufen vor
// dem Laden der Inhalte, am einfachsten in einer init-Funktion.
func RegisterEffect(name string, e Effect) { effects[name] = e }

// EffectNames listet die registrierten Wirkungen, sortiert.
func EffectNames() []string { return sortedKeys(effects) }

func init() {
	// heal: Spieler bekommt "amount" HP (Standard 1), ohne Obergrenze.
	RegisterEffect("heal", func(s *State, _ *Object, call *EffectCall) {
		s.PlayerHP += int(call.Param("amount", 1))
	})
	// damage: Spieler verliert "amount" HP (Standard 1).
	RegisterEffect("damage", func(s *State, o *Object, call *EffectCall) {
		s.hurtPlayer(int(call.Param("amount", 1)))
	})
	// claim: Der Spieler besetzt das Objekt. Es bleibt liegen; hat es sight,
	// sieht er ab jetzt von dort aus.
	RegisterEffect("claim", func(_ *State, o *Object, _ *EffectCall) { o.Owned = true })
	// remove: Das Objekt verschwindet und zählt als eingesammelt.
	RegisterEffect("remove", func(s *State, o *Object, _ *EffectCall) {
		s.Objects = slices.DeleteFunc(s.Objects, func(x *Object) bool { return x == o })
		s.Collected[o.Def.ID]++
	})
}

// enter löst die Wirkungen eines Objekts aus, das der Spieler betreten hat.
func (s *State) enter(o *Object) {
	for i := range o.Def.OnEnter {
		call := &o.Def.OnEnter[i]
		effects[call.Effect](s, o, call)
		if s.Result != Running {
			return
		}
	}
	if o.Def.EnterMessage != "" {
		s.say(o.Def.EnterMessage)
	}
}

// hurtPlayer zieht dem Spieler HP ab. Fällt er auf 0, ist der Einsatz
// verloren. Rückgabe: ob er noch lebt.
func (s *State) hurtPlayer(amount int) bool {
	s.PlayerHP -= amount
	if s.PlayerHP > 0 {
		return true
	}
	s.PlayerHP = 0
	s.Result = Lost
	s.say("Du wurdest besiegt.")
	return false
}
