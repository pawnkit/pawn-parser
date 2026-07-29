package parser_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	parser "github.com/pawnkit/pawn-parser"
)

func TestCompilerAcceptanceDifferential(t *testing.T) {
	t.Parallel()

	compiler := os.Getenv("PAWN_PARSER_PAWNCC")
	corpus := os.Getenv("PAWN_CORPUS_DIR")
	if compiler == "" || corpus == "" {
		t.Skip("set PAWN_PARSER_PAWNCC and PAWN_CORPUS_DIR to run compiler differentials")
	}

	fixtures := []string{
		"syntax/valid/compiler/at_global.pwn",
		"syntax/valid/compiler/block_shadowing.pwn",
		"lexer/compiler_binary_literal.pwn",
		"lexer/compiler_digit_separator.pwn",
		"lexer/compiler_hex_dollar_suffix.pwn",
		"lexer/compiler_string_prefix.pwn",
	}
	for _, relativePath := range fixtures {
		t.Run(filepath.Base(relativePath), func(t *testing.T) {
			t.Parallel()

			source, err := os.ReadFile(filepath.Join(corpus, filepath.FromSlash(relativePath))) //nolint:gosec // The paths are fixed fixtures.
			if err != nil {
				t.Fatal(err)
			}
			parserAccepted := !parser.Parse(source).HasParseErrors()
			compilerAccepted, output := compilerAccepts(t, compiler, source)
			if parserAccepted != compilerAccepted {
				t.Fatalf("acceptance differs: parser=%t compiler=%t\n%s", parserAccepted, compilerAccepted, output)
			}
		})
	}
}

func compilerAccepts(t *testing.T, compiler string, source []byte) (bool, []byte) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.pwn"), source, 0o600); err != nil { //nolint:gosec // dir is a test-owned temporary directory.
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, compiler, "fixture.pwn", "-ofixture.amx") //nolint:gosec // CI supplies the pinned compiler path.
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("pawncc timed out: %v", ctx.Err())
	}
	if err == nil {
		return true, output
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run pawncc: %v", err)
	}

	return false, output
}
