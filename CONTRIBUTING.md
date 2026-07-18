# Contributing

PawnKit is maintained by volunteers, so reviews may take a little time.

Parser bugs are easiest to review when the report includes a short Pawn input
and the tree or token behavior you expected. Small grammar and recovery fixes
are welcome.

Run the project checks before opening a pull request:

```sh
task check
```

Preserve source bytes and trivia. The parser should describe syntax without
making semantic or framework-specific decisions. Add a regression test for
every accepted syntax or recovery change.
