package parser

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pawnkit/pawn-parser/lexer"
)

func TestParseTokensCompactContextCancelled(t *testing.T) {
	t.Parallel()
	source := []byte(strings.Repeat("stock Value() { return 1; }\n", 1000))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	file, err := ParseTokensCompactContext(ctx, source, lexer.Tokenize(source), ParseOptions{})
	if file != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseTokensCompactContext() = (%v, %v)", file, err)
	}
}

func TestParseTokensCompactContextStopsDuringParse(t *testing.T) {
	t.Parallel()
	source := []byte(strings.Repeat("stock Value() { return 1; }\n", 1000))
	ctx := &delayedCancelContext{}
	file, err := ParseTokensCompactContext(ctx, source, lexer.Tokenize(source), ParseOptions{})
	if file != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("ParseTokensCompactContext() = (%v, %v)", file, err)
	}
}

type delayedCancelContext struct {
	checks int
}

func (c *delayedCancelContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *delayedCancelContext) Done() <-chan struct{}       { return nil }
func (c *delayedCancelContext) Value(any) any               { return nil }

func (c *delayedCancelContext) Err() error {
	c.checks++
	if c.checks > 1 {
		return context.Canceled
	}
	return nil
}
