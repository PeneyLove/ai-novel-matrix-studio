package cli

import (
	"strings"
	"unicode/utf8"
)

// latexToUnicode renders a LaTeX math expression as a best-effort Unicode
// approximation suitable for a terminal: Greek letters, operators/relations,
// super/subscripts, \frac, \sqrt and accents map to real glyphs; anything it
// can't represent degrades to a readable plain-text form. There is no Go
// library for this, so the symbol table below is maintained by hand.
func latexToUnicode(expr string) string {
	return convertMath([]rune(expr))
}

func convertMath(rs []rune) string {
	var b strings.Builder
	b.Grow(len(rs))
	for i := 0; i < len(rs); {
		switch r := rs[i]; r {
		case '\\':
			i = convertCommand(&b, rs, i)
		case '^':
			arg, ni := readAtom(rs, i+1, false)
			b.WriteString(superscript(convertMath([]rune(arg))))
			i = ni
		case '_':
			arg, ni := readAtom(rs, i+1, false)
			b.WriteString(subscript(convertMath([]rune(arg))))
			i = ni
		case '{', '}':
			i++
		case '&', '~':
			b.WriteByte(' ')
			i++
		case '$':
			i++
		default:
			b.WriteRune(r)
			i++
		}
	}
	return b.String()
}

// convertCommand consumes the backslash command starting at rs[i] and writes
// its rendering; it returns the index just past everything it consumed.
func convertCommand(b *strings.Builder, rs []rune, i int) int {
	j := i + 1
	if j >= len(rs) {
		return j
	}
	if !isASCIILetter(rs[j]) {
		switch ch := rs[j]; ch {
		case '\\':
			b.WriteString("  ")
		case ',', ';', ':', '!', ' ':
			b.WriteByte(' ')
		default:
			b.WriteRune(ch)
		}
		return j + 1
	}

	k := j
	for k < len(rs) && isASCIILetter(rs[k]) {
		k++
	}
	cmd := string(rs[j:k])

	switch cmd {
	case "frac", "tfrac", "dfrac":
		num, k2 := readAtom(rs, k, true)
		den, k3 := readAtom(rs, k2, true)
		b.WriteString(renderFrac(convertMath([]rune(num)), convertMath([]rune(den))))
		return k3
	case "sqrt":
		idx := ""
		if k < len(rs) && rs[k] == '[' {
			idx, k = readBracket(rs, k)
		}
		arg, k2 := readAtom(rs, k, true)
		b.WriteString(renderSqrt(idx, convertMath([]rune(arg))))
		return k2
	case "text", "textrm", "textbf", "textit", "mathrm", "mathsf", "mathtt", "mathit", "mathbf", "mathcal", "operatorname":
		arg, k2 := readAtom(rs, k, true)
		b.WriteString(arg)
		return k2
	case "mathbb":
		arg, k2 := readAtom(rs, k, true)
		b.WriteString(blackboard(arg))
		return k2
	case "left", "right", "big", "Big", "bigg", "Bigg", "bigl", "bigr", "Bigl", "Bigr", "displaystyle", "textstyle", "limits", "nolimits":
		return k
	case "begin", "end":
		_, k2 := readAtom(rs, k, true)
		return k2
	}

	if combining, ok := accents[cmd]; ok {
		arg, k2 := readAtom(rs, k, true)
		b.WriteString(applyCombining(convertMath([]rune(arg)), combining))
		return k2
	}
	if sym, ok := symbols[cmd]; ok {
		b.WriteString(sym)
		return k
	}
	b.WriteString(cmd)
	return k
}

// readAtom reads the argument of a command or script: a {balanced group}, a
// \command, or a single rune. Returns the inner text (no surrounding braces)
// and the index just past it.
func readAtom(rs []rune, i int, skipSpaces bool) (string, int) {
	if skipSpaces {
		for i < len(rs) && rs[i] == ' ' {
			i++
		}
	}
	if i >= len(rs) {
		return "", i
	}
	switch rs[i] {
	case '{':
		depth := 0
		start := i + 1
		for j := i; j < len(rs); j++ {
			switch rs[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return string(rs[start:j]), j + 1
				}
			}
		}
		return string(rs[start:]), len(rs)
	case '\\':
		k := i + 1
		if k < len(rs) && !isASCIILetter(rs[k]) {
			return string(rs[i : k+1]), k + 1
		}
		for k < len(rs) && isASCIILetter(rs[k]) {
			k++
		}
		return string(rs[i:k]), k
	default:
		return string(rs[i]), i + 1
	}
}

func readBracket(rs []rune, i int) (string, int) {
	start := i + 1
	for j := start; j < len(rs); j++ {
		if rs[j] == ']' {
			return string(rs[start:j]), j + 1
		}
	}
	return "", len(rs)
}

func renderFrac(num, den string) string {
	return wrapIfCompound(num) + "/" + wrapIfCompound(den)
}

func renderSqrt(idx, arg string) string {
	if utf8.RuneCountInString(arg) > 1 {
		arg = "(" + arg + ")"
	}
	switch idx {
	case "", "2":
		return "âˆ? + arg
	case "3":
		return "âˆ? + arg
	case "4":
		return "âˆ? + arg
	}
	return superscript(idx) + "âˆ? + arg
}

func wrapIfCompound(s string) string {
	if utf8.RuneCountInString(s) > 1 {
		return "(" + s + ")"
	}
	return s
}

func applyCombining(s string, mark rune) string {
	rs := []rune(s)
	if len(rs) == 0 {
		return string(mark)
	}
	return string(rs[0]) + string(mark) + string(rs[1:])
}

func blackboard(s string) string {
	var b strings.Builder
	for _, r := range s {
		if bb, ok := blackboardCaps[r]; ok {
			b.WriteRune(bb)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func superscript(s string) string {
	if t, ok := mapAll(s, superMap); ok {
		return t
	}
	if utf8.RuneCountInString(s) == 1 {
		return "^" + s
	}
	return "^(" + s + ")"
}

func subscript(s string) string {
	if t, ok := mapAll(s, subMap); ok {
		return t
	}
	if utf8.RuneCountInString(s) == 1 {
		return "_" + s
	}
	return "_(" + s + ")"
}

func mapAll(s string, m map[rune]rune) (string, bool) {
	if s == "" {
		return "", true
	}
	var b strings.Builder
	for _, r := range s {
		c, ok := m[r]
		if !ok {
			return "", false
		}
		b.WriteRune(c)
	}
	return b.String(), true
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// normalizeMath rewrites the alternate math delimiters \(..\) and \[..\] to
// $..$ / $$..$$ and collapses newlines inside a $$ display block onto one line
// so the inline math parser sees a single contiguous run. It tracks fenced and
// inline code so literal delimiters inside code are never rewritten.
func normalizeMath(s string) string {
	rs := []rune(s)
	n := len(rs)
	var b strings.Builder
	b.Grow(len(s))

	inFenced, inCode, inDisplay := false, false, false

	for i := 0; i < n; {
		r := rs[i]

		if r == '`' && i+2 < n && rs[i+1] == '`' && rs[i+2] == '`' {
			inFenced = !inFenced
			b.WriteString("```")
			i += 3
			continue
		}
		if r == '`' && !inFenced {
			inCode = !inCode
			b.WriteRune(r)
			i++
			continue
		}
		if inFenced || inCode {
			b.WriteRune(r)
			i++
			continue
		}

		if r == '\\' && i+1 < n {
			switch rs[i+1] {
			case '\\':
				b.WriteString("\\\\")
				i += 2
				continue
			case '[':
				b.WriteString("$$")
				inDisplay = true
				i += 2
				continue
			case ']':
				b.WriteString("$$")
				inDisplay = false
				i += 2
				continue
			case '(':
				b.WriteString("$")
				i += 2
				continue
			case ')':
				b.WriteString("$")
				i += 2
				continue
			}
		}
		if r == '$' && i+1 < n && rs[i+1] == '$' {
			b.WriteString("$$")
			inDisplay = !inDisplay
			i += 2
			continue
		}
		if r == '\n' && inDisplay {
			b.WriteByte(' ')
			i++
			continue
		}

		b.WriteRune(r)
		i++
	}
	return b.String()
}

var symbols = map[string]string{
	"alpha": "Î±", "beta": "Î²", "gamma": "Î³", "delta": "Î´", "epsilon": "Îµ",
	"varepsilon": "Îµ", "zeta": "Î¶", "eta": "Î·", "theta": "Î¸", "vartheta": "Ï‘",
	"iota": "Î¹", "kappa": "Îº", "lambda": "Î»", "mu": "Î¼", "nu": "Î½", "xi": "Î¾",
	"omicron": "Î¿", "pi": "Ï€", "varpi": "Ï–", "rho": "Ï", "varrho": "Ï±",
	"sigma": "Ïƒ", "varsigma": "Ï‚", "tau": "Ï„", "upsilon": "Ï…", "phi": "Ï†",
	"varphi": "Ï•", "chi": "Ï‡", "psi": "Ïˆ", "omega": "Ï‰",
	"Gamma": "Î“", "Delta": "Î”", "Theta": "Î˜", "Lambda": "Î›", "Xi": "Îž",
	"Pi": "Î ", "Sigma": "Î£", "Upsilon": "Î¥", "Phi": "Î¦", "Psi": "Î¨", "Omega": "Î©",

	"times": "Ã—", "div": "Ã·", "cdot": "Â·", "ast": "âˆ?, "star": "â‹?,
	"pm": "Â±", "mp": "âˆ?, "oplus": "âŠ?, "ominus": "âŠ?, "otimes": "âŠ?,
	"oslash": "âŠ?, "odot": "âŠ?, "circ": "âˆ?, "bullet": "â€?, "setminus": "âˆ?,

	"leq": "â‰?, "le": "â‰?, "geq": "â‰?, "ge": "â‰?, "neq": "â‰?, "ne": "â‰?,
	"equiv": "â‰?, "approx": "â‰?, "cong": "â‰?, "sim": "âˆ?, "simeq": "â‰?,
	"propto": "âˆ?, "ll": "â‰?, "gg": "â‰?, "doteq": "â‰?, "asymp": "â‰?,

	"leftarrow": "â†?, "rightarrow": "â†?, "to": "â†?, "gets": "â†?,
	"leftrightarrow": "â†?, "Leftarrow": "â‡?, "Rightarrow": "â‡?,
	"Leftrightarrow": "â‡?, "implies": "â‡?, "iff": "â‡?, "mapsto": "â†?,
	"uparrow": "â†?, "downarrow": "â†?, "longrightarrow": "âŸ?, "longleftarrow": "âŸ?,

	"sum": "âˆ?, "prod": "âˆ?, "coprod": "âˆ?, "int": "âˆ?, "iint": "âˆ?,
	"iiint": "âˆ?, "oint": "âˆ?, "nabla": "âˆ?, "partial": "âˆ?,
	"infty": "âˆ?, "sqrt": "âˆ?, "surd": "âˆ?,

	"in": "âˆ?, "notin": "âˆ?, "ni": "âˆ?, "subset": "âŠ?, "supset": "âŠ?,
	"subseteq": "âŠ?, "supseteq": "âŠ?, "cup": "âˆ?, "cap": "âˆ?,
	"emptyset": "âˆ?, "varnothing": "âˆ?, "forall": "âˆ€", "exists": "âˆ?,
	"nexists": "âˆ?, "neg": "Â¬", "lnot": "Â¬", "land": "âˆ?, "wedge": "âˆ?,
	"lor": "âˆ?, "vee": "âˆ?,

	"angle": "âˆ?, "perp": "âŠ?, "parallel": "âˆ?, "mid": "âˆ?, "nmid": "âˆ?,
	"triangle": "â–?, "square": "â–?, "diamond": "â—?, "top": "âŠ?, "bot": "âŠ?,
	"vdash": "âŠ?, "models": "âŠ?, "therefore": "âˆ?, "because": "âˆ?,

	"ldots": "â€?, "dots": "â€?, "cdots": "â‹?, "vdots": "â‹?, "ddots": "â‹?,
	"prime": "â€?, "degree": "Â°", "deg": "Â°", "hbar": "â„?, "ell": "â„?,
	"Re": "â„?, "Im": "â„?, "aleph": "â„?, "wp": "â„?,
	"langle": "âŸ?, "rangle": "âŸ?, "lceil": "âŒ?, "rceil": "âŒ?,
	"lfloor": "âŒ?, "rfloor": "âŒ?, "backslash": "\\",

	"quad": "  ", "qquad": "    ", "space": " ", "thinspace": " ",
	"lim": "lim", "sin": "sin", "cos": "cos", "tan": "tan", "log": "log",
	"ln": "ln", "exp": "exp", "min": "min", "max": "max", "det": "det",
	"gcd": "gcd", "dim": "dim", "ker": "ker",
}

var accents = map[string]rune{
	"hat": 'Ì‚', "widehat": 'Ì‚', "bar": 'Ì„', "overline": 'Ì„',
	"vec": 'âƒ?, "dot": 'Ì‡', "ddot": 'Ìˆ', "tilde": 'Ìƒ',
	"widetilde": 'Ìƒ', "acute": 'Ì', "grave": 'Ì€', "check": 'ÌŒ',
}

var superMap = map[rune]rune{
	'0': 'â?, '1': 'Â¹', '2': 'Â²', '3': 'Â³', '4': 'â?, '5': 'â?, '6': 'â?,
	'7': 'â?, '8': 'â?, '9': 'â?, '+': 'â?, '-': 'â?, '=': 'â?, '(': 'â?,
	')': 'â?, 'a': 'áµ?, 'b': 'áµ?, 'c': 'á¶?, 'd': 'áµ?, 'e': 'áµ?, 'f': 'á¶?,
	'g': 'áµ?, 'h': 'Ê°', 'i': 'â?, 'j': 'Ê²', 'k': 'áµ?, 'l': 'Ë¡', 'm': 'áµ?,
	'n': 'â?, 'o': 'áµ?, 'p': 'áµ?, 'r': 'Ê³', 's': 'Ë¢', 't': 'áµ?, 'u': 'áµ?,
	'v': 'áµ?, 'w': 'Ê·', 'x': 'Ë£', 'y': 'Ê¸', 'z': 'á¶?,
}

var subMap = map[rune]rune{
	'0': 'â‚€', '1': 'â‚?, '2': 'â‚?, '3': 'â‚?, '4': 'â‚?, '5': 'â‚?, '6': 'â‚?,
	'7': 'â‚?, '8': 'â‚?, '9': 'â‚?, '+': 'â‚?, '-': 'â‚?, '=': 'â‚?, '(': 'â‚?,
	')': 'â‚?, 'a': 'â‚?, 'e': 'â‚?, 'h': 'â‚?, 'i': 'áµ?, 'j': 'â±?, 'k': 'â‚?,
	'l': 'â‚?, 'm': 'â‚?, 'n': 'â‚?, 'o': 'â‚?, 'p': 'â‚?, 'r': 'áµ?, 's': 'â‚?,
	't': 'â‚?, 'u': 'áµ?, 'v': 'áµ?, 'x': 'â‚?,
}

var blackboardCaps = map[rune]rune{
	'A': 'ð”¸', 'B': 'ð”¹', 'C': 'â„?, 'D': 'ð”»', 'E': 'ð”¼', 'F': 'ð”½', 'G': 'ð”¾',
	'H': 'â„?, 'I': 'ð•€', 'J': 'ð•', 'K': 'ð•‚', 'L': 'ð•ƒ', 'M': 'ð•„', 'N': 'â„?,
	'O': 'ð•†', 'P': 'â„?, 'Q': 'â„?, 'R': 'â„?, 'S': 'ð•Š', 'T': 'ð•‹', 'U': 'ð•Œ',
	'V': 'ð•', 'W': 'ð•Ž', 'X': 'ð•', 'Y': 'ð•', 'Z': 'â„?,
}
