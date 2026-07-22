# Changelog

## 1.1.8 - 2026-07-22

### Fixed

- Include colons in generic and macro tag ranges.

## 1.1.7 - 2026-07-22

### Fixed

- Preserve symbolic sizes in packed array dimensions.

## 1.1.6 - 2026-07-22

### Fixed

- Preserve conditional regions inside call argument lists.

## 1.1.5 - 2026-07-22

### Fixed

- Preserve `char` markers in packed array dimensions.

## 1.1.4 - 2026-07-21

### Fixed

- Avoid stale diagnostics from conditional `else if` splices.

## 1.1.3 - 2026-07-21

### Fixed

- Parse parameterized tags and operator arguments used by PawnPlus declaration macros.

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
