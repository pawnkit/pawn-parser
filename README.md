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

- `github.com/pawnkit/pawn-parser` — Pawn CST parser and node kinds.
- `github.com/pawnkit/pawn-parser/lexer` — standalone tokenizer.
- `github.com/pawnkit/pawn-parser/token` — token kinds, positions, and trivia.