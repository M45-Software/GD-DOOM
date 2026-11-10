package dehacked

import (
	"fmt"
	"reflect"
	"slices"
)

// Apply resolves patches in order and returns an independent definitions set.
// Neither successful nor failed application can mutate the base or any Patch.
// Pointer copies always read the compiled actions, not previous patch results.
func (d *Definitions) Apply(patches ...*Patch) (*Definitions, error) {
	if d == nil || len(d.data.Things) != NumThings || len(d.data.States) != NumStates {
		return nil, fmt.Errorf("DeHackEd definitions must start from Vanilla()")
	}
	next := &Definitions{data: cloneSnapshot(d.data)}
	for _, patch := range patches {
		if patch == nil {
			return nil, fmt.Errorf("cannot apply a nil DeHackEd patch")
		}
		for _, c := range patch.changes {
			section, member := c.section, c.member
			if section == "pointer" || section == "codeptr" {
				section, member = "frame", "Action"
			}
			next.data.Defined[fieldKey(section, c.index, member)] = true
			switch c.section {
			case "strings":
				next.data.Strings[c.member] = c.text
			case "pars":
				next.data.Pars[c.member] = int32(c.number)
			case "cheat":
				next.data.Cheats[c.member] = c.text
			case "codeptr":
				next.data.States[c.index].Action = Action(c.text)
			case "pointer":
				next.data.States[c.index].Action = vanillaStates[c.number].Action
			case "text":
				next.applyText(c.old, c.text)
			default:
				var target any
				switch c.section {
				case "thing":
					target = &next.data.Things[c.index]
				case "frame":
					target = &next.data.States[c.index]
				case "weapon":
					target = &next.data.Weapons[c.index]
				case "ammo":
					target = &next.data.Ammo[c.index]
				case "sound":
					target = &next.data.Sounds[c.index]
				case "misc":
					target = &next.data.Misc
				default:
					return nil, Diagnostic{c.where.Source, c.where.Line, "invalid parsed patch operation"}
				}
				field := reflect.ValueOf(target).Elem().FieldByName(c.member)
				if c.kind == flagsField {
					field.SetUint(uint64(c.number))
				} else {
					field.SetInt(c.number)
				}
			}
		}
	}
	return next, nil
}

func (d *Definitions) applyText(old, text string) {
	if len(old) == 4 && len(text) == 4 {
		for i, name := range d.data.Sprites {
			if !d.FieldWasSet("sprite", i, "Name") && lower(name) == lower(old) {
				d.data.Sprites[i] = text
				d.data.Defined[fieldKey("sprite", i, "Name")] = true
				return
			}
		}
	}
	if len(old) > 0 && len(old) < 7 && len(text) < 7 {
		for i, sound := range d.data.Sounds {
			if i > 0 && !d.FieldWasSet("sound", i, "Name") && lower(sound.Name) == lower(old) {
				d.data.Sounds[i].Name = text
				d.data.Defined[fieldKey("sound", i, "Name")] = true
				return
			}
		}
		for i, name := range d.data.Music {
			if i > 0 && !d.FieldWasSet("music", i, "Name") && lower(name) == lower(old) {
				d.data.Music[i] = text
				d.data.Defined[fieldKey("music", i, "Name")] = true
				return
			}
		}
	}
	// A resource is renameable only once, across the whole patch stack. A
	// later attempt must not escape through the generic text replacement map.
	if len(old) == 4 && len(text) == 4 {
		for i, name := range vanillaSprites {
			if d.FieldWasSet("sprite", i, "Name") && (lower(name) == lower(old) || lower(d.data.Sprites[i]) == lower(old)) {
				return
			}
		}
	}
	if len(old) > 0 && len(old) < 7 && len(text) < 7 {
		for i, sound := range vanillaSounds {
			if i > 0 && d.FieldWasSet("sound", i, "Name") && (lower(sound.Name) == lower(old) || lower(d.data.Sounds[i].Name) == lower(old)) {
				return
			}
		}
		for i, name := range vanillaMusic {
			if i > 0 && d.FieldWasSet("music", i, "Name") && (lower(name) == lower(old) || lower(d.data.Music[i]) == lower(old)) {
				return
			}
		}
	}
	// Keep non-resource literal replacements too: the eventual presentation
	// consumer may contain original strings outside the English macro catalog.
	d.data.TextReplacements[old] = text
	// Matching is against compiled values, so a later patch can override an
	// earlier replacement using the same original string.
	keys := make([]string, 0, len(vanillaStrings))
	for key := range vanillaStrings {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if lower(vanillaStrings[key]) == lower(old) {
			d.data.Strings[key] = formatString(text)
			d.data.Defined[fieldKey("strings", 0, key)] = true
		}
	}
}
