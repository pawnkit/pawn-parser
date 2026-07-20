# pawn-parser

Reusable Go lexer and concrete-syntax-tree parser for the Pawn language used by
SA-MP and open.mp projects.

The parser preserves source byte ranges, tokens, comments, whitespace trivia,
preprocessor directives, and conditional compilation regions.

## Install

```sh
go get github.com/pawnkit/pawn-parser
```

## Parse source

```go
package main

import (
	"fmt"

	parser "github.com/pawnkit/pawn-parser"
)

func main() {
	source := []byte("stock AddOne(value) { return value + 1; }")
	file := parser.Parse(source)
	if file.HasParseErrors() {
		panic("invalid Pawn source")
	}

	fmt.Println(file.Root.Kind)
	fmt.Println(file.Root.Text(source))
}
```

## Packages

- `github.com/pawnkit/pawn-parser`: Pawn CST parser and node kinds
- `github.com/pawnkit/pawn-parser/lexer`: standalone tokenizer
- `github.com/pawnkit/pawn-parser/token`: token kinds, positions, and trivia

## Parse profiles

Use `ParseWithProfile` for new compact consumers:

- `ProfileLossless` retains syntax, tokens, trivia, and origins for formatting.
- `ProfileAnalysis` retains compact syntax and diagnostics for linting.
- `ProfileTokensOnly` tokenizes without building a syntax tree.

`Parse` remains the pointer-tree compatibility API. `ParseForLinter` remains an
alias for the analysis profile.

Analysis consumers can use typed, allocation-free traversal:

```go
file := parser.ParseWithProfile(source, parser.ProfileAnalysis)
declarations := file.Syntax().Declarations()
for declarations.Next() {
	function, ok := parser.AsFunction(declarations.Declaration())
	if !ok {
		continue
	}
	name, _ := function.Name()
	fmt.Println(name.Text())
}
```

Formatters should use `ProfileLossless`. Syntax token handles then expose
retained leading trivia, trailing trivia, and origin chains without expanding
the pointer CST.

The v1.0 pointer-tree API remains supported. See
[compatibility](docs/compatibility.md) before choosing an API for a new
integration.

## Contributing

Grammar fixes and small compatibility cases are welcome. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the test and review expectations.
