package preprocess

import (
	"maps"
	"strconv"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
)

// MacroKind distinguishes object-like from function-like macros.
type MacroKind uint8

const (
	// MacroObjectLike has no invocation arguments.
	MacroObjectLike MacroKind = iota + 1
	// MacroFunctionLike accepts invocation arguments.
	MacroFunctionLike
)

func (k MacroKind) String() string {
	if k == MacroFunctionLike {
		return "function-like"
	}
	return "object-like"
}

// Macro is one active #define.
type Macro struct {
	Name string
	// Pattern is Pawn's decoded textual match pattern. Unlike a C macro,
	// everything after Name up to the first unescaped whitespace participates
	// in matching, including fixed punctuation around %0-%9 captures.
	Pattern         string
	Kind            MacroKind
	ParamCount      int
	ParamSlots      map[int]int
	NamedParams     map[string]int
	FlexiblePattern bool
	Body            []ptok
	// BodyText retains the exact substitution spelling. Pawn substitution is
	// textual, so re-lexing this text after inserting captures is required for
	// token pasting such as "__%0".
	BodyText string
	BodySpan ByteRange
	File     uint32
	DefSpan  ByteRange
}

func macroNameToken(item token.Token, source []byte) bool {
	text := item.Text(source)
	if text == "" {
		return false
	}
	first := text[0]
	return first == '_' || first == '@' || first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
}

// ReplacementCallable returns the function called by a forwarding macro.
func (m Macro) ReplacementCallable() (string, bool) {
	if m.FlexiblePattern {
		name, lowest := "", m.ParamCount
		for candidate, slot := range m.NamedParams {
			if slot < lowest {
				name, lowest = candidate, slot
			}
		}
		if name != "" {
			return name, true
		}
	}
	for index := range len(m.Body) {
		if m.Body[index].Kind != token.Identifier {
			continue
		}
		next := index + 1
		for next < len(m.Body) && m.Body[next].Kind.IsTrivia() {
			next++
		}
		if next < len(m.Body) && m.Body[next].Kind == token.LParen {
			return m.Body[index].text, true
		}
	}
	return "", false
}

type macroTable struct {
	byName        map[string]Macro
	maxNameLength int
}

func newMacroTable(maxNameLength int) *macroTable {
	return &macroTable{byName: make(map[string]Macro), maxNameLength: maxNameLength}
}

func (t *macroTable) name(name string) string {
	if t != nil && t.maxNameLength > 0 && len(name) > t.maxNameLength {
		return name[:t.maxNameLength]
	}
	return name
}

func (t *macroTable) define(m Macro) (previous Macro, redefined bool) {
	m.Name = t.name(m.Name)
	previous, redefined = t.byName[m.Name]
	t.byName[m.Name] = m
	return previous, redefined
}

func (t *macroTable) undef(name string) bool {
	name = t.name(name)
	_, ok := t.byName[name]
	delete(t.byName, name)
	return ok
}

func (t *macroTable) lookup(name string) (Macro, bool) {
	name = t.name(name)
	m, ok := t.byName[name]
	return m, ok
}

func (t *macroTable) defined(name string) bool {
	name = t.name(name)
	_, ok := t.byName[name]
	return ok
}

// snapshot returns an immutable copy of the current macro definitions,
// safe to retain after processing continues to mutate the table.
func (t *macroTable) snapshot() map[string]Macro {
	out := make(map[string]Macro, len(t.byName))
	maps.Copy(out, t.byName)
	return out
}

// parseParamIndex parses a positional label such as "%0".
func parseParamIndex(text string) (index int, ok bool) {
	if text == "%%" {
		return 0, false
	}
	digits := strings.TrimPrefix(text, "%")
	if digits == "" {
		return 0, false
	}
	n := 0
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

func patternParamSlots(pattern string) (map[int]int, int) {
	slots := make(map[int]int)
	count := 0
	for index := 0; index+1 < len(pattern); index++ {
		if pattern[index] != '%' || pattern[index+1] < '0' || pattern[index+1] > '9' {
			continue
		}
		label := int(pattern[index+1] - '0')
		if _, exists := slots[label]; !exists {
			slots[label] = count
			count++
		}
		index++
	}
	return slots, count
}

func namedPatternParams(name, pattern string) map[string]int {
	if strings.Contains(pattern, "%") || !strings.HasPrefix(pattern, name+"(") || !strings.HasSuffix(pattern, ")") {
		return nil
	}
	inside := pattern[len(name)+1 : len(pattern)-1]
	if inside == "" {
		return nil
	}
	parts := strings.Split(inside, ",")
	params := make(map[string]int, len(parts))
	for index, part := range parts {
		if part == "" || !macroNameText(part) {
			return nil
		}
		params[part] = index
	}
	return params
}

func macroNameText(text string) bool {
	if text == "" {
		return false
	}
	for index, char := range []byte(text) {
		if index == 0 {
			if !pawnAlpha(char) {
				return false
			}
		} else if !pawnAlphanum(char) {
			return false
		}
	}
	return true
}

func macroPrefixText(text string) string {
	end := 0
	for end < len(text) && pawnAlphanum(text[end]) {
		end++
	}
	if end == 0 || !pawnAlpha(text[0]) {
		return ""
	}
	return text[:end]
}

func pawnAlpha(char byte) bool {
	return char == '_' || char == '@' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func pawnAlphanum(char byte) bool {
	return pawnAlpha(char) || char >= '0' && char <= '9'
}

//nolint:gocyclo // Pawn pattern escapes are decoded in one scan.
func decodePawnPattern(raw []byte, control byte) string {
	var output strings.Builder
	output.Grow(len(raw))
	for index := 0; index < len(raw); index++ {
		char := raw[index]
		if char != control || index+1 >= len(raw) {
			output.WriteByte(char)
			continue
		}
		index++
		escaped := raw[index]
		switch escaped {
		case control:
			output.WriteByte(control)
		case 'a':
			output.WriteByte(7)
		case 'b':
			output.WriteByte(8)
		case 'e':
			output.WriteByte(27)
		case 'f':
			output.WriteByte(12)
		case 'n':
			output.WriteByte('\n')
		case 'r':
			output.WriteByte('\r')
		case 't':
			output.WriteByte('\t')
		case 'v':
			output.WriteByte(11)
		case '\'', '"', '%':
			output.WriteByte(escaped)
		case 'x':
			start := index + 1
			end := start
			for end < len(raw) && (raw[end] >= '0' && raw[end] <= '9' || raw[end] >= 'a' && raw[end] <= 'f' || raw[end] >= 'A' && raw[end] <= 'F') {
				end++
			}
			if value, err := strconv.ParseUint(string(raw[start:end]), 16, 8); err == nil && end > start {
				output.WriteByte(byte(value))
				index = end - 1
				if end < len(raw) && raw[end] == ';' {
					index = end
				}
			} else {
				output.WriteByte(escaped)
			}
		default:
			if escaped >= '0' && escaped <= '9' {
				start := index
				end := start
				for end < len(raw) && raw[end] >= '0' && raw[end] <= '9' {
					end++
				}
				if value, err := strconv.ParseUint(string(raw[start:end]), 10, 8); err == nil {
					output.WriteByte(byte(value))
				}
				index = end - 1
				if end < len(raw) && raw[end] == ';' {
					index = end
				}
			} else {
				// PawnCC diagnoses this escape. Retaining its spelling keeps the
				// tooling preprocessor lossless after the diagnostic frontier.
				output.WriteByte(escaped)
			}
		}
	}
	return output.String()
}

func macroBodyText(source []byte, tokens []token.Token) (string, ByteRange) {
	if len(tokens) == 0 {
		return "", ByteRange{}
	}
	start := tokens[0].Start.Offset
	end := tokens[len(tokens)-1].End.Offset
	if start < 0 || start > end || end > len(source) {
		return "", ByteRange{}
	}
	body := strings.TrimRight(string(source[start:end]), " \t\r\n")
	return body, ByteRange{Start: start, End: start + len(body)}
}
