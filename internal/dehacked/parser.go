package dehacked

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// This parser targets classic text patch format 6 and core Boom/BEX syntax.
// Binary executable patches, MBF/DEHEXTRA/MBF21 and their extra actions/slots
// are reported as unsupported, never silently interpreted as vanilla data.
const (
	DefaultMaxBytes        = 16 << 20
	DefaultMaxChanges      = 100000
	DefaultMaxIncludeDepth = 16
)

type Diagnostic struct {
	Source  string
	Line    int
	Message string
}

func (d Diagnostic) Error() string { return fmt.Sprintf("%s:%d: %s", d.Source, d.Line, d.Message) }

// Source contains the bytes returned by an explicit include resolver. Parse
// never opens files, interprets paths or accesses the network itself.
type Source struct {
	Name string
	Data []byte
}
type SourceIdentity struct{ Name, SHA256 string }

type ParseOptions struct {
	ResolveInclude                        func(includingSource, name string) (Source, error)
	MaxBytes, MaxChanges, MaxIncludeDepth int
}

// Patch owns its parsed operations. Its accessors return copies, making a Patch
// reusable concurrently for different immutable definition snapshots.
type Patch struct {
	changes     []change
	diagnostics []Diagnostic
	sources     []SourceIdentity
}

func (p *Patch) Diagnostics() []Diagnostic { return append([]Diagnostic(nil), p.diagnostics...) }
func (p *Patch) Sources() []SourceIdentity { return append([]SourceIdentity(nil), p.sources...) }
func (p *Patch) ChangeCount() int          { return len(p.changes) }

type change struct {
	where     Diagnostic
	section   string
	index     int
	member    string
	number    int64
	text, old string
	kind      fieldKind
}

type parser struct {
	options ParseOptions
	patch   *Patch
	bytes   int
	stack   map[string]bool
}

type cursor struct {
	source    string
	data      []byte
	pos, line int
}

func (c *cursor) nextLine() (string, int, bool) {
	if c.pos == len(c.data) {
		return "", c.line, false
	}
	start, line := c.pos, c.line
	for c.pos < len(c.data) && c.data[c.pos] != '\n' {
		c.pos++
	}
	end := c.pos
	if c.pos < len(c.data) {
		c.pos++
		c.line++
	}
	return strings.TrimSuffix(string(c.data[start:end]), "\r"), line, true
}

func trim(s string) string { return strings.Trim(s, " \t\r\n\v\f") }
func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] -= 'a' - 'A'
		}
	}
	return string(b)
}

var (
	blockHeader   = regexp.MustCompile(`(?i)^(Thing|Frame|Sound|Ammo|Weapon|Sprite|Cheat|Misc)\s+([+-]?(?:0x[0-9a-f]+|[0-9]+))(?:\s+\(.*\))?\s*$`)
	pointerHeader = regexp.MustCompile(`(?i)^Pointer\s+([+-]?(?:0x[0-9a-f]+|[0-9]+))\s+\(Frame\s+([+-]?(?:0x[0-9a-f]+|[0-9]+))\)\s*$`)
	identifier    = regexp.MustCompile(`^[A-Za-z_][A-Za-z_0-9]*$`)
)

func Parse(source string, data []byte) (*Patch, error) {
	return ParseWithOptions(source, data, ParseOptions{})
}

func ParseWithOptions(source string, data []byte, options ParseOptions) (*Patch, error) {
	if options.MaxBytes == 0 {
		options.MaxBytes = DefaultMaxBytes
	}
	if options.MaxChanges == 0 {
		options.MaxChanges = DefaultMaxChanges
	}
	if options.MaxIncludeDepth == 0 {
		options.MaxIncludeDepth = DefaultMaxIncludeDepth
	}
	if options.MaxBytes < 1 || options.MaxChanges < 1 || options.MaxIncludeDepth < 1 {
		return nil, fmt.Errorf("DeHackEd parser limits must be positive")
	}
	if source == "" {
		source = "<patch>"
	}
	p := &parser{options: options, patch: &Patch{}, stack: map[string]bool{}}
	if err := p.parse(Source{source, data}, false, 0); err != nil {
		return nil, err
	}
	return p.patch, nil
}

func (p *parser) fail(source string, line int, format string, args ...any) error {
	return Diagnostic{source, line, fmt.Sprintf(format, args...)}
}

func (p *parser) add(c change) error {
	if len(p.patch.changes) >= p.options.MaxChanges {
		return p.fail(c.where.Source, c.where.Line, "patch change limit exceeded")
	}
	p.patch.changes = append(p.patch.changes, c)
	return nil
}

func (p *parser) warn(d Diagnostic) error {
	if len(p.patch.diagnostics) >= p.options.MaxChanges {
		return p.fail(d.Source, d.Line, "patch diagnostic limit exceeded")
	}
	p.patch.diagnostics = append(p.patch.diagnostics, d)
	return nil
}

func (p *parser) parse(source Source, skipText bool, depth int) error {
	if depth >= p.options.MaxIncludeDepth {
		return p.fail(source.Name, 1, "INCLUDE depth limit exceeded")
	}
	if p.stack[source.Name] {
		return p.fail(source.Name, 1, "INCLUDE cycle")
	}
	if len(p.patch.sources) >= p.options.MaxChanges {
		return p.fail(source.Name, 1, "patch source-count limit exceeded")
	}
	if len(source.Data) > p.options.MaxBytes-p.bytes {
		return p.fail(source.Name, 1, "patch byte limit exceeded")
	}
	if i := strings.IndexByte(string(source.Data), 0); i >= 0 {
		return p.fail(source.Name, 1+strings.Count(string(source.Data[:i]), "\n"), "NUL byte at offset %d; binary patches are unsupported", i)
	}
	p.bytes += len(source.Data)
	p.stack[source.Name] = true
	defer delete(p.stack, source.Name)
	digest := sha256.Sum256(source.Data)
	p.patch.sources = append(p.patch.sources, SourceIdentity{source.Name, hex.EncodeToString(digest[:])})
	c := &cursor{source: source.Name, data: source.Data, line: 1}
	section, index := "", 0
	for {
		raw, line, ok := c.nextLine()
		if !ok {
			break
		}
		text := trim(raw)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		folded := lower(text)
		where := Diagnostic{Source: source.Name, Line: line}
		if strings.HasPrefix(folded, "patch file for dehacked ") {
			if folded != "patch file for dehacked v3.0" && folded != "patch file for dehacked v2.3" {
				return p.fail(source.Name, line, "unsupported DeHackEd text header %q", text)
			}
			section = ""
			continue
		}
		if strings.HasPrefix(folded, "include ") || strings.HasPrefix(folded, "include\t") {
			rest := trim(text[len("include"):])
			notext := skipText
			if strings.HasPrefix(lower(rest), "notext ") || strings.HasPrefix(lower(rest), "notext\t") {
				notext = true
				rest = trim(rest[len("notext"):])
			}
			if strings.HasPrefix(rest, `"`) {
				if len(rest) < 2 || !strings.HasSuffix(rest, `"`) {
					return p.fail(source.Name, line, "unterminated INCLUDE filename")
				}
				rest = rest[1 : len(rest)-1]
			}
			if rest == "" || p.options.ResolveInclude == nil {
				return p.fail(source.Name, line, "INCLUDE requires a filename and an explicit resolver")
			}
			included, err := p.options.ResolveInclude(source.Name, rest)
			if err != nil {
				return p.fail(source.Name, line, "INCLUDE %q: %v", rest, err)
			}
			if included.Name == "" {
				return p.fail(source.Name, line, "INCLUDE resolver returned an empty source identity")
			}
			if err = p.parse(included, notext, depth+1); err != nil {
				return err
			}
			section = ""
			continue
		}
		if strings.HasPrefix(folded, "text ") || strings.HasPrefix(folded, "text\t") {
			if skipText {
				// Boom's NOTEXT handler skips lines until EOF or a line starting
				// with a literal space. Blank lines do not stop it. Preserve this
				// observable parser quirk rather than executing skipped sections.
				if err := p.warn(Diagnostic{source.Name, line, "INCLUDE NOTEXT skips through the next space-leading line or EOF, following Boom's legacy parser"}); err != nil {
					return err
				}
				for c.pos < len(c.data) {
					start := c.pos
					c.nextLine()
					if c.data[start] == ' ' {
						break
					}
				}
				section = ""
				continue
			}
			words := strings.Fields(text)
			if len(words) != 3 {
				return p.fail(source.Name, line, "Text requires old and new byte lengths")
			}
			from, e1 := parseInt(words[1])
			to, e2 := parseInt(words[2])
			if e1 != nil || e2 != nil || from < 0 || to < 0 || from > int64(p.options.MaxBytes) || to > int64(p.options.MaxBytes)-from {
				return p.fail(source.Name, line, "invalid Text byte lengths")
			}
			payload := make([]byte, 0, int(from+to))
			for len(payload) < int(from+to) && c.pos < len(c.data) {
				b := c.data[c.pos]
				c.pos++
				if b == '\n' {
					c.line++
				}
				if b != '\r' {
					payload = append(payload, b)
				}
			}
			if len(payload) != int(from+to) {
				return p.fail(source.Name, line, "truncated Text payload: expected %d bytes, got %d", from+to, len(payload))
			}
			if !skipText {
				if err := p.add(change{where: where, section: "text", old: string(payload[:int(from)]), text: string(payload[int(from):])}); err != nil {
					return err
				}
			}
			section = ""
			continue
		}
		if strings.HasPrefix(text, "[") {
			switch folded {
			case "[strings]":
				section = "strings"
			case "[pars]":
				section = "pars"
			case "[codeptr]":
				section = "codeptr"
			default:
				return p.fail(source.Name, line, "unsupported BEX block %q (core Boom/BEX only)", text)
			}
			continue
		}
		if match := pointerHeader.FindStringSubmatch(text); match != nil {
			ordinal, err := parseInt(match[1])
			if err != nil || ordinal < 0 {
				return p.fail(source.Name, line, "invalid Pointer ordinal")
			}
			n, err := parseInt(match[2])
			if err != nil || n < 0 || n >= NumStates {
				return p.fail(source.Name, line, "Pointer target frame out of range")
			}
			section, index = "pointer", int(n)
			continue
		}
		if match := blockHeader.FindStringSubmatch(text); match != nil {
			n, err := parseInt(match[2])
			if err != nil {
				return p.fail(source.Name, line, "invalid section index")
			}
			section = lower(match[1])
			limit := map[string]int64{"thing": NumThings, "frame": NumStates, "sound": NumSounds, "ammo": NumAmmo, "weapon": NumWeapons, "sprite": NumSprites, "cheat": 1, "misc": 1}[section]
			if section == "thing" {
				n--
			}
			if n < 0 || n >= limit {
				return p.fail(source.Name, line, "%s index out of range", match[1])
			}
			index = int(n)
			continue
		}
		if section == "pars" {
			if err := p.par(where, text); err != nil {
				return err
			}
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return p.fail(source.Name, line, "expected a section header or field assignment")
		}
		key, value = trim(key), trim(value)
		field := lower(key)
		if section == "" {
			switch field {
			case "doom version", "patch format":
				n, err := parseInt(value)
				if err != nil || field == "doom version" && n != 19 && n != 21 || field == "patch format" && n != 6 {
					return p.fail(source.Name, line, "unsupported %s %q", key, value)
				}
			case "engine config", "iwad": // WhackEd metadata, not load instructions.
			default:
				return p.fail(source.Name, line, "assignment outside a supported block: %s", key)
			}
			continue
		}
		if section == "strings" {
			if !identifier.MatchString(key) {
				return p.fail(source.Name, line, "invalid BEX string key %q", key)
			}
			for strings.HasSuffix(value, `\`) {
				value = strings.TrimSuffix(value, `\`)
				next, _, ok := c.nextLine()
				if !ok {
					return p.fail(source.Name, line, "unterminated BEX string continuation")
				}
				for strings.HasPrefix(trim(next), "#") {
					next, _, ok = c.nextLine()
					if !ok {
						return p.fail(source.Name, line, "unterminated BEX string continuation")
					}
				}
				value += strings.TrimRight(strings.TrimLeft(next, " \t"), " \t\r")
			}
			if _, known := vanillaStrings[upper(key)]; !known {
				if err := p.warn(Diagnostic{source.Name, line, "string key " + key + " has no vanilla baseline; retained for the consumer"}); err != nil {
					return err
				}
			}
			if err := p.add(change{where: where, section: section, member: upper(key), text: formatString(value)}); err != nil {
				return err
			}
			continue
		}
		if section == "codeptr" {
			words := strings.Fields(key)
			if len(words) != 2 || lower(words[0]) != "frame" {
				return p.fail(source.Name, line, "expected FRAME number in [CODEPTR]")
			}
			n, err := parseInt(words[1])
			if err != nil || n < 0 || n >= NumStates {
				return p.fail(source.Name, line, "CODEPTR frame out of range")
			}
			action, ok := findAction(value)
			if !ok {
				return p.fail(source.Name, line, "unknown or unsupported action %q", value)
			}
			if err := p.add(change{where: where, section: section, index: int(n), text: string(action)}); err != nil {
				return err
			}
			continue
		}
		if section == "cheat" {
			if _, known := vanillaCheats[field]; !known {
				return p.fail(source.Name, line, "unknown cheat %q", key)
			}
			if end := strings.IndexByte(value, 0xff); end >= 0 {
				value = value[:end]
			}
			if err := p.add(change{where: where, section: section, member: field, text: value}); err != nil {
				return err
			}
			continue
		}
		spec, known := fieldSpecs[section][field]
		if !known {
			return p.fail(source.Name, line, "unknown or unsupported %s field %q", section, key)
		}
		if spec.kind == ignoredField {
			if err := p.warn(Diagnostic{source.Name, line, "legacy " + section + " field " + key + " is ignored by the Boom parser"}); err != nil {
				return err
			}
			continue
		}
		n, err := parseInt(value)
		if spec.kind == flagsField {
			var bits uint32
			bits, err = parseFlags(value)
			n = int64(bits)
		}
		if err != nil {
			return p.fail(source.Name, line, "invalid %s value %q: %v", key, value, err)
		}
		limit := int64(0)
		switch spec.kind {
		case stateField, pointerField:
			limit = NumStates
		case soundField:
			limit = NumSounds
		case spriteField:
			limit = NumSprites
		case ammoField:
			limit = 6
		}
		if limit > 0 && (n < 0 || n >= limit || spec.kind == ammoField && n == 4) {
			return p.fail(source.Name, line, "%s reference out of range: %d", key, n)
		}
		if section == "misc" && spec.member == "MonstersInfight" && n != 202 && n != 221 {
			return p.fail(source.Name, line, "Monsters Infight must be 202 or 221")
		}
		if err := p.add(change{where: where, section: section, index: index, member: spec.member, number: n, kind: spec.kind}); err != nil {
			return err
		}
	}
	return nil
}

// Accept a complete 32-bit word in decimal/hex/octal, retaining two's-complement
// encodings such as 0xffffffff for -1. Reject overflow and trailing junk rather
// than relying on the C parser's scanf overflow or partial-conversion behavior.
func parseInt(text string) (int64, error) {
	n, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		return 0, err
	}
	if n < -1<<31 || n > 1<<32-1 {
		return 0, fmt.Errorf("value does not fit a 32-bit word")
	}
	return int64(int32(n)), nil
}

func parseFlags(text string) (uint32, error) {
	if n, err := strconv.ParseInt(text, 0, 32); err == nil {
		return uint32(n), nil
	}
	if n, err := strconv.ParseUint(text, 0, 32); err == nil {
		return uint32(n), nil
	}
	var bits uint32
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == '+' || r == '|' || r == ',' || r == ' ' || r == '\t' })
	if len(parts) == 0 {
		return 0, fmt.Errorf("empty Bits value")
	}
	for _, part := range parts {
		value, ok := thingFlags[upper(part)]
		if !ok {
			return 0, fmt.Errorf("unknown or unsupported flag %q", part)
		}
		bits |= value
	}
	return bits, nil
}

func findAction(text string) (Action, bool) {
	name := upper(trim(text))
	if name == "NULL" {
		return NoAction, true
	}
	for _, s := range vanillaStates {
		if upper(string(s.Action)) == name {
			return s.Action, true
		}
	}
	return "", false
}

func formatString(value string) string {
	return strings.NewReplacer(`\n`, "\n", `\N`, "\n").Replace(value)
}

func (p *parser) par(where Diagnostic, text string) error {
	words := strings.Fields(text)
	if (len(words) != 3 && len(words) != 4) || lower(words[0]) != "par" {
		return p.fail(where.Source, where.Line, "expected par [episode] map seconds")
	}
	numbers := make([]int64, len(words)-1)
	for i, word := range words[1:] {
		n, err := parseInt(word)
		if err != nil || n < 0 {
			return p.fail(where.Source, where.Line, "invalid par value %q", word)
		}
		numbers[i] = n
	}
	var name string
	if len(numbers) == 2 {
		if numbers[0] < 1 || numbers[0] > 32 {
			return p.fail(where.Source, where.Line, "par map must be 1..32")
		}
		name = fmt.Sprintf("MAP%02d", numbers[0])
	} else {
		if numbers[0] < 1 || numbers[0] > 3 || numbers[1] < 1 || numbers[1] > 9 {
			return p.fail(where.Source, where.Line, "Boom episode par requires E1M1..E3M9")
		}
		name = fmt.Sprintf("E%dM%d", numbers[0], numbers[1])
	}
	return p.add(change{where: where, section: "pars", member: name, number: numbers[len(numbers)-1]})
}
