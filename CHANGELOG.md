# Changelog

## 1.1.2 - 2026-07-21

### Fixed

- Parse generated declarations, macro-based tags, and keyword-named pattern macros used by common Pawn includes.

## 1.1.1 - 2026-07-21

### Fixed

- Parsed `@` callback declarations with `const` array parameters without leaking errors from annotation lookahead.

## 1.1.0 - 2026-07-20

### Added

- Compact parse profiles for formatters, analysis tools, and token-only users.
- Immutable typed syntax adapters with allocation-free traversal.
- Structured syntax diagnostics and recovery details.
- Parsing from preprocessed token streams.
- Compact token positions, trivia, origins, and line maps.
- Conformance coverage from `pawn-corpus` and recorded performance baselines.

### Changed

- Reduced allocations in the lexer, parser, and analysis profile.
- Improved recovery around macros, directives, declarations, arrays, tags, and
  conditional syntax.
- Kept `Parse` and the original pointer-tree API source-compatible with v1.0.0.

[1.1.0]: https://github.com/pawnkit/pawn-parser/compare/v1.0.0...v1.1.0
