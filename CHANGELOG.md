# Changelog

## 1.5.11 - 2026-08-03

### Performance

- Rebase declaration indexes after local body edits without rehashing unchanged declarations.

## 1.5.10 - 2026-08-03

### Added

- Preserve compact token and trivia data when parsing an existing token stream.
- Add a cancellable compact tokenizer for incremental consumers.

## 1.5.9 - 2026-08-02

### Changed

- Use the current `pawnkit-core` v0.5.0 release.

## 1.5.8 - 2026-08-02

### Added

- Added a bounded Pawn preprocessor for directives, macros, conditionals, and
  include resolution, with source mappings and cancellation support.
- Added preprocessor reuse and token-cache helpers for incremental consumers.

## 1.5.7 - 2026-08-01

### Fixed

- Set the Pawn compiler library path in the differential CI job.

## 1.5.6 - 2026-07-30

### Performance

- Reparse a single changed top-level declaration when the surrounding syntax is unchanged.

## 1.5.5 - 2026-07-29

### Performance

- Reduced compact syntax memory growth on large files.
- Added a performance budget for compact parsing.

## 1.5.4 - 2026-07-29

### Changed

- Reduced compact expansion memory by removing its token lookup map.

## 1.5.3 - 2026-07-29

### Added

- Expand compact syntax with an existing token stream.

## 1.5.2 - 2026-07-29

### Changed

- Reduced allocations when expanding compact syntax.

## 1.5.1 - 2026-07-29

### Added

- Expand compact syntax with the caller's token and trivia retention options.

## 1.5.0 - 2026-07-29

### Added

- Rebase clean compact trees after grammar-neutral trivia edits.

## 1.4.2 - 2026-07-29

### Added

- Compare parser and pawncc acceptance for shared compiler probes.

## 1.4.1 - 2026-07-26

### Fixed

- Keep function bodies available inside conditional regions after syntax errors.

## 1.4.0 - 2026-07-26

### Added

- Added cancellable tokenization for editor analysis.

## 1.3.0 - 2026-07-26

### Added

- Added cancellable compact parsing for editor analysis.

## 1.2.0 - 2026-07-26

### Added

- Added stable top-level declaration boundaries for incremental tools.

## 1.1.10 - 2026-07-25

### Added

- Documented the stable Go API and added a compatibility compile test.
- Published the repository support record.

## 1.1.9 - 2026-07-22

### Fixed

- Parse object-like statement macros without semicolons.
- Handle compact modulo expressions such as `value%4`.
- Preserve inline operator macros and multiline PawnPlus syntax.

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
