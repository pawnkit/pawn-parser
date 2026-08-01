package preprocess

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pawnkit/pawn-parser/token"
	"github.com/pawnkit/pawnkit-core/source"
)

// ListingOptions describes the command-line state written at the start of a
// Pawn compiler listing. A listing is requested by assigning this structure
// to [Options.Listing].
type ListingOptions struct {
	ControlChar byte
	Packed      bool
	Semicolons  bool
	TabSize     int
}

func (o ListingOptions) resolved() ListingOptions {
	if o.ControlChar == 0 {
		o.ControlChar = '\\'
	}
	if o.TabSize <= 0 {
		o.TabSize = 8
	}
	return o
}

type listingEventKind uint8

const (
	listingFile listingEventKind = iota + 1
	listingLine
)

type listingEvent struct {
	kind         listingEventKind
	file         uint32
	line         int
	sourceLine   bool
	raw          []byte
	tokens       []ptok
	replacements []ByteRange
	joinNext     bool
}

func (e *engine) recordListingFile(file uint32) {
	if e.opts.Listing == nil {
		return
	}
	e.listingCode = -1
	e.listing = append(e.listing, listingEvent{kind: listingFile, file: file})
}

func (e *engine) recordListingGaps(frame *frame, before int) {
	if e.opts.Listing == nil || before <= frame.listedLine+1 {
		return
	}
	for line := frame.listedLine + 1; line < before; line++ {
		if listingCommentContinuationLine(frame, line, e.opts.ControlChar) {
			continue
		}
		e.listing = append(e.listing, listingEvent{kind: listingLine, file: frame.fileIndex, line: adjustedListingLine(frame, line)})
	}
	frame.listedLine = before - 1
}

func (e *engine) recordListingDirective(frame *frame) {
	if e.opts.Listing == nil || !frame.currentActive() || frame.pos+1 >= len(frame.toks) {
		return
	}
	keyword := frame.toks[frame.pos+1]
	switch classifyDirective(keyword.Text(frame.source)) {
	case dirPragma, dirLine, dirFile, dirEmit, dirEndinput:
		line := adjustedListingLine(frame, frame.cur().Start.Line)
		start, end := sourceLineBounds(frame.source, frame.cur().Start.Offset)
		e.listing = append(e.listing, listingEvent{
			kind: listingLine, file: frame.fileIndex, line: line,
			raw: stripListingComments(frame.source[start:end]),
		})
	}
}

func (e *engine) beginListingCode(frame *frame, line int) {
	if e.opts.Listing == nil || !frame.currentActive() || frame.listedLine == line || e.listingCode >= 0 {
		return
	}
	e.listing = append(e.listing, listingEvent{kind: listingLine, file: frame.fileIndex, line: adjustedListingLine(frame, line), sourceLine: true})
	e.listingCode = len(e.listing) - 1
}

func (e *engine) recordListingTail(frame *frame) {
	if e.opts.Listing == nil || frame.endinput || e.stopped {
		return
	}
	lines := bytes.Count(frame.source, []byte{'\n'})
	if len(frame.source) != 0 {
		lines++
	}
	for line := frame.listedLine + 1; line <= lines; line++ {
		e.listing = append(e.listing, listingEvent{kind: listingLine, file: frame.fileIndex, line: adjustedListingLine(frame, line)})
	}
	e.listingCode = -1
}

func adjustedListingLine(frame *frame, line int) int {
	if frame == nil {
		return line
	}
	return max(1, line-frame.listingLineAdjustment)
}

func renderListing(files []FileInfo, events []listingEvent, options ListingOptions) []byte {
	var output bytes.Buffer
	lineEndings := listingLineEndings(files)
	fmt.Fprintf(&output, "#pragma ctrlchar 0x%02x\n", options.ControlChar)
	fmt.Fprintf(&output, "#pragma pack %s\n", strconv.FormatBool(options.Packed))
	fmt.Fprintf(&output, "#pragma semicolon %s\n", strconv.FormatBool(options.Semicolons))
	fmt.Fprintf(&output, "#pragma tabsize %d\n", options.TabSize)

	currentFile := ^uint32(0)
	listedLine := -1
	for _, event := range events {
		switch event.kind {
		case listingFile:
			separator := true
			if int(currentFile) < len(files) && int(event.file) < len(files) && files[event.file].Depth < files[currentFile].Depth {
				content := files[currentFile].Content
				separator = len(content) == 0 || content[len(content)-1] == '\n'
			}
			currentFile = event.file
			listedLine = -1
			if separator {
				output.WriteByte('\n')
			}
			writeListingFilename(&output, listingFilename(files, event.file))
		case listingLine:
			if event.file != currentFile {
				currentFile = event.file
				listedLine = -1
				output.WriteByte('\n')
				writeListingFilename(&output, listingFilename(files, event.file))
			}
			if event.line != listedLine+1 {
				fmt.Fprintf(&output, "#line %d\n", event.line)
			}
			listedLine = event.line
			switch {
			case len(event.tokens) != 0:
				output.Write(renderListingLine(files, event))
			case event.raw != nil:
				output.Write(event.raw)
			case event.sourceLine:
				output.Write(renderEmptyListingLine(files, event))
			}
			if !event.joinNext {
				preserveSourceEnding := len(event.tokens) != 0 || event.raw != nil
				if preserveSourceEnding && int(event.file) < len(lineEndings) && event.line > 0 && event.line <= len(lineEndings[event.file]) && lineEndings[event.file][event.line-1] {
					output.WriteString("\r\n")
				} else {
					output.WriteByte('\n')
				}
			}
		}
	}
	if int(currentFile) < len(files) {
		content := files[currentFile].Content
		if len(content) != 0 && content[len(content)-1] != '\n' {
			listing := output.Bytes()
			if len(listing) != 0 && listing[len(listing)-1] == '\n' {
				output.Truncate(len(listing) - 1)
			}
		}
	}
	return output.Bytes()
}

func listingLineStarts(source []byte, enabled bool) []uint32 {
	if !enabled {
		return nil
	}
	return token.NewLineMap(source).Starts
}

func listingCommentContinuationLine(frame *frame, line int, controlChar byte) bool {
	if frame == nil || line <= 0 || line > len(frame.lineStarts) || controlChar == 0 {
		return false
	}
	start := int(frame.lineStarts[line-1])
	end := len(frame.source)
	if line < len(frame.lineStarts) {
		end = int(frame.lineStarts[line]) - 1
	}
	body := bytes.TrimSpace(frame.source[start:end])
	return bytes.HasPrefix(body, []byte("/*")) && len(body) != 0 && body[len(body)-1] == controlChar
}

func renderEmptyListingLine(files []FileInfo, event listingEvent) []byte {
	if int(event.file) >= len(files) || len(event.replacements) == 0 {
		return nil
	}
	content := files[event.file].Content
	lineStart, lineEnd := sourceLineBounds(content, event.replacements[0].Start)
	var output bytes.Buffer
	writeListingGap(&output, content, lineStart, lineEnd, event.replacements)
	return output.Bytes()
}

func listingLineEndings(files []FileInfo) [][]bool {
	endings := make([][]bool, len(files))
	for file, info := range files {
		for index, character := range info.Content {
			if character == '\n' {
				endings[file] = append(endings[file], index > 0 && info.Content[index-1] == '\r')
			}
		}
		if len(info.Content) != 0 && info.Content[len(info.Content)-1] != '\n' {
			endings[file] = append(endings[file], false)
		}
	}
	return endings
}

func writeListingFilename(output *bytes.Buffer, filename string) {
	output.WriteString("#file \"")
	output.WriteString(filename)
	output.WriteString("\"\n")
}

func (e *engine) joinListingWithNextLine() {
	if e.listingCode >= 0 && e.listingCode < len(e.listing) {
		e.listing[e.listingCode].joinNext = true
	}
}

func listingFilename(files []FileInfo, file uint32) string {
	if int(file) >= len(files) {
		return ""
	}
	if files[file].ListingPath != "" {
		return files[file].ListingPath
	}
	name := files[file].URI
	if filename, err := source.URI(name).Filename(); err == nil {
		return filename
	}
	return name
}

func renderListingLine(files []FileInfo, event listingEvent) []byte {
	fileID := event.file
	items := event.tokens
	if len(items) == 0 || int(fileID) >= len(files) {
		return nil
	}
	content := files[fileID].Content
	firstSpan := listingSpan(items[0])
	lineStart, lineEnd := sourceLineBounds(content, firstSpan.Start.Offset)
	var output bytes.Buffer
	if firstSpan.File == fileID && lineStart <= firstSpan.Start.Offset {
		writeListingGap(&output, content, lineStart, firstSpan.Start.Offset, event.replacements)
	}
	for index, item := range items {
		span := listingSpan(item)
		if index != 0 {
			previous := listingSpan(items[index-1])
			if continuation, ok := listingContinuation(items[index-1].trailing); ok {
				output.WriteString(continuation)
			} else if items[index-1].Origin == nil && item.Origin == nil && previous.File == fileID && span.File == fileID && previous.End.Offset <= span.Start.Offset {
				writeListingGap(&output, content, previous.End.Offset, span.Start.Offset, event.replacements)
			} else if separator := listingSeparator(items[index-1], item); separator != "" {
				output.WriteString(separator)
			}
		}
		spelling, _ := listingContinuationText(item.text)
		output.WriteString(spelling)
	}
	lastSpan := listingSpan(items[len(items)-1])
	if lastSpan.File == fileID && lastSpan.End.Offset <= lineEnd {
		writeListingGap(&output, content, lastSpan.End.Offset, lineEnd, event.replacements)
	}
	return output.Bytes()
}

func listingSeparator(previous, current ptok) string {
	if marker, ok := listingContinuation(previous.trailing); ok {
		return marker
	}
	if whitespace := listingHorizontalWhitespace(previous.trailing) + listingHorizontalWhitespace(current.leading); whitespace != "" {
		return whitespace
	}
	for _, trivia := range current.LeadingTrivia {
		if trivia.Kind == token.Whitespace {
			return " "
		}
	}
	if previous.Origin == nil || current.Origin == nil || previous.Origin.Macro != current.Origin.Macro {
		return ""
	}
	for _, trivia := range previous.TrailingTrivia {
		if trivia.Kind == token.Whitespace {
			return " "
		}
	}
	return ""
}

func listingHorizontalWhitespace(trailing string) string {
	end := 0
	for end < len(trailing) && (trailing[end] == ' ' || trailing[end] == '\t') {
		end++
	}
	return trailing[:end]
}

func listingContinuation(trailing string) (string, bool) {
	newline := strings.IndexAny(trailing, "\r\n")
	if newline < 0 {
		return "", false
	}
	end := newline
	for end > 0 && (trailing[end-1] == ' ' || trailing[end-1] == '\t') {
		end--
	}
	if end == 0 || trailing[end-1] != '\\' {
		return "", false
	}
	return trailing[:end-1] + "\a", true
}

func listingContinuationText(text string) (string, bool) {
	var output strings.Builder
	changed := false
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\\' {
			continue
		}
		newline := index + 1
		for newline < len(text) && (text[newline] == ' ' || text[newline] == '\t') {
			newline++
		}
		lineEnd := newline
		if lineEnd < len(text) && text[lineEnd] == '\r' {
			lineEnd++
		}
		if lineEnd >= len(text) || text[lineEnd] != '\n' {
			continue
		}
		output.WriteString(text[start:index])
		output.WriteByte('\a')
		lineEnd++
		for lineEnd < len(text) && (text[lineEnd] == ' ' || text[lineEnd] == '\t') {
			lineEnd++
		}
		start = lineEnd
		index = lineEnd - 1
		changed = true
	}
	if !changed {
		return text, false
	}
	output.WriteString(text[start:])
	return output.String(), true
}

func writeListingGap(output *bytes.Buffer, content []byte, start, end int, replacements []ByteRange) {
	position := start
	for _, replacement := range replacements {
		if replacement.End <= position || replacement.Start >= end {
			continue
		}
		if replacement.Start > position {
			output.Write(stripListingComments(content[position:min(replacement.Start, end)]))
		}
		position = max(position, replacement.End)
		if position >= end {
			return
		}
	}
	if position < end {
		output.Write(stripListingComments(content[position:end]))
	}
}

func listingSpan(item ptok) token.Span {
	if item.listed != nil {
		return *item.listed
	}
	if item.Origin != nil {
		return item.Origin.Span
	}
	return token.Span{File: item.file, Start: item.Start, End: item.End}
}

func sourceLineBounds(content []byte, offset int) (int, int) {
	offset = min(max(offset, 0), len(content))
	start := bytes.LastIndexByte(content[:offset], '\n') + 1
	end := len(content)
	if relative := bytes.IndexByte(content[offset:], '\n'); relative >= 0 {
		end = offset + relative
	}
	if end > start && content[end-1] == '\r' {
		end--
	}
	return start, end
}

func stripListingComments(content []byte) []byte {
	output := bytes.Clone(content)
	for index := 0; index < len(output); {
		switch {
		case index+1 < len(output) && output[index] == '/' && output[index+1] == '/':
			return output[:index]
		case index+1 < len(output) && output[index] == '/' && output[index+1] == '*':
			output[index], output[index+1] = ' ', ' '
			index += 2
			closed := false
			for index < len(output) {
				if index+1 < len(output) && output[index] == '*' && output[index+1] == '/' {
					output[index], output[index+1] = ' ', ' '
					index += 2
					closed = true
					break
				}
				output[index] = ' '
				index++
			}
			if !closed {
				// PawnCC replaces the source newline while inside a block comment
				// with a space before the following empty comment line terminates
				// the listing line.
				output = append(output, ' ')
			}
		default:
			index++
		}
	}
	return output
}
