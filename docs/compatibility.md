# Compatibility

`pawn-parser` follows semantic versioning. The public API is stable within a
major version.

Version 1.1 adds compact parsing and typed traversal without removing the v1.0
pointer-tree API. Existing calls to `Parse`, the lexer package, and token types
continue to compile unchanged.

New analysis tools should use `ParseWithProfile` with `ProfileAnalysis`.
Formatters that need every token and comment should use `ProfileLossless`.
`Parse` remains useful for integrations built around `*Node`.

## API lifecycle

The root `parser` package and the `lexer` and `token` packages are stable.
Their exported parsing entry points, token and syntax kinds, byte ranges,
diagnostics, and documented traversal types follow semantic versioning.

Numeric kind values are process-local implementation details. Store kind names
or your own identifiers when data crosses a process or persists to disk.
Recovery trees for invalid source may improve between minor releases.

Everything under `internal` is internal and has no compatibility promise.
New exported helpers start as preview when their release notes say so; they
become stable only after this document names them.

`RebaseCompactTrivia` is a preview helper for incremental analysis. It returns
false when an edit could change the grammar, so callers must keep a full-parse
fallback.

Each minor release is checked with Go's `apidiff` against the previous stable
tag. Parser output may become more accurate in minor releases: valid syntax can
produce a better tree, and invalid syntax can produce different recovery nodes
or diagnostics. Consumers should match node kinds instead of relying on the
exact shape of an error-recovery tree.

## Known gaps

- A missing statement semicolon is recorded on the syntax node but does not add
  a file diagnostic.
- An unterminated preprocessor conditional at end of file is not yet diagnosed.

The shared corpus test tracks both cases. They remain outside the passing
conformance set until their diagnostic behavior is settled.
