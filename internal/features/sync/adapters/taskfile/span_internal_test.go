// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package taskfile

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	yaml "go.yaml.in/yaml/v3"
)

type (
	closingQuoteCase struct {
		content []byte
		want    int
		quote   byte
	}

	quoteStyleCase struct {
		style      yaml.Style
		wantQuote  byte
		wantQuoted bool
	}

	spanCase struct {
		start     int
		end       int
		wantStart int
	}

	positionCase struct {
		content []byte
		line    int
		column  int
	}
)

const (
	wantErrText  = "expected error"
	oldPathValue = "../pnpm/Taskfile.yml"
	spanFmt      = "span = %d..%d, want %d..%d"
	twoLines     = "first\nsecond\n"
	otherLine    = "other\n"
	taskfileKey  = "taskfile: "
)

// TestQuoteForYAMLStyleMapsQuotedStyles verifies each scalar style maps to its quote byte.
func TestQuoteForYAMLStyleMapsQuotedStyles(t *testing.T) {
	t.Parallel()

	cases := []quoteStyleCase{
		{style: yaml.DoubleQuotedStyle, wantQuote: '"', wantQuoted: true},
		{style: yaml.SingleQuotedStyle, wantQuote: '\'', wantQuoted: true},
		{style: yaml.LiteralStyle, wantQuote: byte(consts.IndexZero), wantQuoted: false},
	}

	for i := range cases {
		assertQuote(t, &cases[i])
	}
}

// TestPlainScalarValueSpanFindsPath verifies the plain scalar span covers the old path.
func TestPlainScalarValueSpanFindsPath(t *testing.T) {
	t.Parallel()

	content := []byte(taskfileKey + oldPathValue + "\n")
	offset := len(taskfileKey)

	start, end, err := plainScalarValueSpan(content, offset, oldPathValue)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	assertSpan(t, &spanCase{start: start, end: end, wantStart: offset})
}

// TestPlainScalarValueSpanReportsMissingPath verifies a mismatched path is reported.
func TestPlainScalarValueSpanReportsMissingPath(t *testing.T) {
	t.Parallel()

	start, end, err := plainScalarValueSpan(
		[]byte(taskfileKey+otherLine),
		consts.IndexZero,
		oldPathValue,
	)
	iox.Discard2(start, end)
	assertFails(t, err)
}

// TestQuotedScalarValueSpanFindsInterior verifies double and single quoted spans.
func TestQuotedScalarValueSpanFindsInterior(t *testing.T) {
	t.Parallel()

	assertQuotedSpan(t, '"')
	assertQuotedSpan(t, '\'')
}

// TestQuotedScalarValueSpanRejectsUnquotedScalar verifies a missing opening quote fails.
func TestQuotedScalarValueSpanRejectsUnquotedScalar(t *testing.T) {
	t.Parallel()

	assertQuotedSpanFails(t, []byte(oldPathValue))
	assertQuotedSpanFails(t, []byte(consts.Empty))
}

// TestQuotedScalarValueSpanReportsUnterminatedQuote verifies a missing closing quote fails.
func TestQuotedScalarValueSpanReportsUnterminatedQuote(t *testing.T) {
	t.Parallel()
	assertQuotedSpanFails(t, []byte(`"`+oldPathValue))
}

// TestQuotedScalarValueSpanReportsPathMismatch verifies a different quoted value fails.
func TestQuotedScalarValueSpanReportsPathMismatch(t *testing.T) {
	t.Parallel()
	assertQuotedSpanFails(t, []byte(`"other"`))
}

// TestFindClosingQuoteSkipsEscapes verifies escaped quotes do not terminate the scalar.
func TestFindClosingQuoteSkipsEscapes(t *testing.T) {
	t.Parallel()

	cases := []closingQuoteCase{
		{content: []byte(`"a\"b"`), quote: '"', want: consts.IndexOne + len(`a\"b`)},
		{content: []byte(`'a''b'`), quote: '\'', want: consts.IndexOne + len(`a''b`)},
	}

	for i := range cases {
		assertClosingQuote(t, &cases[i])
	}
}

// TestOffsetAtLineColumnRejectsInvalidPositions verifies out-of-range positions fail.
func TestOffsetAtLineColumnRejectsInvalidPositions(t *testing.T) {
	t.Parallel()

	content := []byte(twoLines)

	cases := []positionCase{
		{content: content, line: consts.IndexZero, column: consts.IndexOne},
		{content: content, line: consts.IndexOne, column: consts.IndexZero},
		{content: content, line: consts.Index99, column: consts.IndexOne},
		{content: content, line: consts.IndexOne, column: consts.Index99},
	}

	for i := range cases {
		assertOffsetFails(t, &cases[i])
	}
}

// TestOffsetAtLineColumnResolvesPosition verifies a valid position maps to a byte offset.
func TestOffsetAtLineColumnResolvesPosition(t *testing.T) {
	t.Parallel()

	offset, err := offsetAtLineColumn(&yamlPosition{
		content: []byte(twoLines),
		line:    consts.IndexTwo,
		column:  consts.IndexOne,
	})
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	wantOffset := len(twoLines) - len("second\n")

	if offset != wantOffset {
		t.Fatalf("offset = %d, want %d", offset, wantOffset)
	}
}

// TestLineEndOffsetHandlesMissingNewline verifies content without a trailing newline.
func TestLineEndOffsetHandlesMissingNewline(t *testing.T) {
	t.Parallel()

	content := []byte("no newline")

	if got := lineEndOffset(content, consts.IndexZero); got != len(content) {
		t.Fatalf("lineEndOffset() = %d, want %d", got, len(content))
	}
}

// TestScalarValueSpanReportsQuotedFailure verifies quoted span failures propagate.
func TestScalarValueSpanReportsQuotedFailure(t *testing.T) {
	t.Parallel()

	start, end, err := scalarValueSpan(&scalarSpanParams{
		content: []byte(oldPathValue),
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		style:   yaml.DoubleQuotedStyle,
	})
	iox.Discard2(start, end)
	assertFails(t, err)
}

// TestSpanFromReplacementReportsFailure verifies span resolution failures propagate.
func TestSpanFromReplacementReportsFailure(t *testing.T) {
	t.Parallel()

	span, err := spanFromReplacement([]byte(otherLine), consts.IndexZero, newReplacement())
	iox.Discard(span)
	assertFails(t, err)
}

// TestIncludePathSpanForReplacementReportsFailures verifies offset and span failures propagate.
func TestIncludePathSpanForReplacementReportsFailures(t *testing.T) {
	t.Parallel()

	replacement := newReplacement()

	replacement.line = consts.IndexZero

	span, err := includePathSpanForReplacement([]byte(otherLine), replacement)
	iox.Discard(span)
	assertFails(t, err)

	span, err = includePathSpanForReplacement([]byte(otherLine), newReplacement())
	iox.Discard(span)
	assertFails(t, err)
}

func assertClosingQuote(t *testing.T, testCase *closingQuoteCase) {
	t.Helper()

	idx, err := findClosingQuote(testCase.content, consts.IndexZero, testCase.quote)
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	if idx != testCase.want {
		t.Fatalf("closing quote = %d, want %d", idx, testCase.want)
	}
}

func assertFails(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal(wantErrText)
	}
}

func assertOffsetFails(t *testing.T, testCase *positionCase) {
	t.Helper()

	offset, err := offsetAtLineColumn(&yamlPosition{
		content: testCase.content,
		line:    testCase.line,
		column:  testCase.column,
	})
	iox.Discard(offset)
	assertFails(t, err)
}

func assertQuote(t *testing.T, testCase *quoteStyleCase) {
	t.Helper()

	quote, quoted := quoteForYAMLStyle(testCase.style)

	if quote != testCase.wantQuote || quoted != testCase.wantQuoted {
		t.Fatalf("quoteForYAMLStyle(%v) = %q, %t", testCase.style, quote, quoted)
	}
}

func assertQuotedSpan(t *testing.T, quote byte) {
	t.Helper()

	content := []byte(string(quote) + oldPathValue + string(quote))

	start, end, err := quotedScalarValueSpan(&quotedSpanParams{
		content: content,
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		quote:   quote,
	})
	if err != nil {
		t.Fatalf(consts.UnexpectedErr, err)
	}

	assertSpan(t, &spanCase{start: start, end: end, wantStart: consts.IndexOne})
}

func assertQuotedSpanFails(t *testing.T, content []byte) {
	t.Helper()

	start, end, err := quotedScalarValueSpan(&quotedSpanParams{
		content: content,
		offset:  consts.IndexZero,
		oldPath: oldPathValue,
		quote:   '"',
	})
	iox.Discard2(start, end)
	assertFails(t, err)
}

func assertSpan(t *testing.T, testCase *spanCase) {
	t.Helper()

	wantEnd := testCase.wantStart + len(oldPathValue)

	if testCase.start != testCase.wantStart || testCase.end != wantEnd {
		t.Fatalf(spanFmt, testCase.start, testCase.end, testCase.wantStart, wantEnd)
	}
}

func newReplacement() *includePathReplacement {
	return &includePathReplacement{
		oldPath: oldPathValue,
		newPath: "../npm/Taskfile.yml",
		line:    consts.IndexOne,
		column:  consts.IndexOne,
		style:   yaml.Style(consts.IndexZero),
	}
}
