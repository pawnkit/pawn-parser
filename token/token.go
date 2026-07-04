// Package token defines token kinds, positions, and trivia shared by the
// lexer and parser packages.
package token

// Kind identifies the lexical category of a token.
type Kind uint16

const (
	// Invalid is the zero value of Kind, used for uninitialized tokens.
	Invalid Kind = iota
	EOF
	Unknown // unrecognized byte(s); the lexer never panics, it emits this instead

	Identifier

	// Literals
	IntLiteral    // 123, 0x1F, 0b101
	FloatLiteral  // 1.0, 1.5e10
	CharLiteral   // 'a', '\n'
	StringLiteral // "text"
	PackedString  // !"text"
	MacroParam    // %0 .. %9, %%

	// Keywords
	KwPublic
	KwStock
	KwStatic
	KwNative
	KwForward
	KwConst
	KwNew
	KwDecl
	KwEnum
	KwSwitch
	KwCase
	KwDefault
	KwIf
	KwElse
	KwFor
	KwWhile
	KwDo
	KwReturn
	KwBreak
	KwContinue
	KwGoto
	KwSizeof
	KwTagof
	KwState
	KwDefined
	KwOperator
	KwNull

	// Operators / punctuation
	Plus          // +
	Minus         // -
	Star          // *
	Slash         // /
	Percent       // %
	Assign        // =
	PlusAssign    // +=
	MinusAssign   // -=
	StarAssign    // *=
	SlashAssign   // /=
	PercentAssign // %=
	ShlAssign     // <<=
	ShrAssign     // >>=
	UshrAssign    // >>>=
	AndAssign     // &=
	OrAssign      // |=
	XorAssign     // ^=
	Eq            // ==
	NotEq         // !=
	Lt            // <
	Gt            // >
	LtEq          // <=
	GtEq          // >=
	Shl           // <<
	Shr           // >>
	Ushr          // >>>
	AndAnd        // &&
	OrOr          // ||
	Amp           // &
	Pipe          // |
	Caret         // ^
	Tilde         // ~
	Bang          // !
	PlusPlus      // ++
	MinusMinus    // --
	Question      // ?
	Colon         // :
	ColonColon    // ::
	Semicolon     // ;
	Comma         // ,
	Dot           // .
	DotDot        // ..
	Ellipsis      // ...
	LParen        // (
	RParen        // )
	LBracket      // [
	RBracket      // ]
	LBrace        // {
	RBrace        // }
	Hash          // #
	At            // @

	// Trivia
	Comment
	Whitespace
	Newline
	LineContinuation
)

var keywords = map[string]Kind{
	"public":   KwPublic,
	"stock":    KwStock,
	"static":   KwStatic,
	"native":   KwNative,
	"forward":  KwForward,
	"const":    KwConst,
	"new":      KwNew,
	"decl":     KwDecl,
	"enum":     KwEnum,
	"switch":   KwSwitch,
	"case":     KwCase,
	"default":  KwDefault,
	"if":       KwIf,
	"else":     KwElse,
	"for":      KwFor,
	"while":    KwWhile,
	"do":       KwDo,
	"return":   KwReturn,
	"break":    KwBreak,
	"continue": KwContinue,
	"goto":     KwGoto,
	"sizeof":   KwSizeof,
	"tagof":    KwTagof,
	"state":    KwState,
	"defined":  KwDefined,
	"operator": KwOperator,
	"null":     KwNull,
}

// LookupKeyword reports the Kind for text if it is a reserved keyword.
func LookupKeyword(text string) (Kind, bool) {
	k, ok := keywords[text]
	return k, ok
}

// IsTrivia reports whether k is a comment, whitespace, newline, or line
// continuation.
func (k Kind) IsTrivia() bool {
	switch k {
	case Comment, Whitespace, Newline, LineContinuation:
		return true
	default:
		return false
	}
}

var kindNames = map[Kind]string{
	Invalid: "Invalid", EOF: "EOF", Unknown: "Unknown",
	Identifier: "Identifier",
	IntLiteral: "IntLiteral", FloatLiteral: "FloatLiteral", CharLiteral: "CharLiteral",
	StringLiteral: "StringLiteral", PackedString: "PackedString", MacroParam: "MacroParam",
	KwPublic: "public", KwStock: "stock", KwStatic: "static", KwNative: "native",
	KwForward: "forward", KwConst: "const", KwNew: "new", KwDecl: "decl", KwEnum: "enum",
	KwSwitch: "switch", KwCase: "case", KwDefault: "default", KwIf: "if", KwElse: "else",
	KwFor: "for", KwWhile: "while", KwDo: "do", KwReturn: "return", KwBreak: "break",
	KwContinue: "continue", KwGoto: "goto", KwSizeof: "sizeof", KwTagof: "tagof",
	KwState: "state", KwDefined: "defined", KwOperator: "operator", KwNull: "null",
	Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%", Assign: "=",
	PlusAssign: "+=", MinusAssign: "-=", StarAssign: "*=", SlashAssign: "/=",
	PercentAssign: "%=", ShlAssign: "<<=", ShrAssign: ">>=", UshrAssign: ">>>=", AndAssign: "&=",
	OrAssign: "|=", XorAssign: "^=", Eq: "==", NotEq: "!=", Lt: "<", Gt: ">",
	LtEq: "<=", GtEq: ">=", Shl: "<<", Shr: ">>", Ushr: ">>>", AndAnd: "&&", OrOr: "||",
	Amp: "&", Pipe: "|", Caret: "^", Tilde: "~", Bang: "!", PlusPlus: "++",
	MinusMinus: "--", Question: "?", Colon: ":", ColonColon: "::", Semicolon: ";",
	Comma: ",", Dot: ".", DotDot: "..", Ellipsis: "...", LParen: "(", RParen: ")",
	LBracket: "[", RBracket: "]", LBrace: "{", RBrace: "}", Hash: "#", At: "@",
	Comment: "Comment", Whitespace: "Whitespace", Newline: "Newline",
	LineContinuation: "LineContinuation",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "Kind(?)"
}

// Position identifies a byte offset and its line/column in a source file.
type Position struct {
	Offset int
	Line   int
	Col    int
}

// Token is a single lexical token together with its surrounding trivia.
type Token struct {
	Kind           Kind
	Start          Position
	End            Position
	LeadingTrivia  []Trivia
	TrailingTrivia []Trivia
}

// Text returns t's exact source text.
func (t Token) Text(source []byte) string {
	if t.Start.Offset < 0 || t.End.Offset > len(source) || t.Start.Offset > t.End.Offset {
		return ""
	}
	return string(source[t.Start.Offset:t.End.Offset])
}

// Trivia is a non-semantic span of source text (whitespace/comment)
// attached to a neighboring Token.
type Trivia struct {
	Kind  Kind
	Start Position
	End   Position
}

// Text returns t's exact source text.
func (t Trivia) Text(source []byte) string {
	if t.Start.Offset < 0 || t.End.Offset > len(source) || t.Start.Offset > t.End.Offset {
		return ""
	}
	return string(source[t.Start.Offset:t.End.Offset])
}

// IsBlankLine reports whether t is a newline trivia.
func (t Trivia) IsBlankLine() bool {
	return t.Kind == Newline
}
