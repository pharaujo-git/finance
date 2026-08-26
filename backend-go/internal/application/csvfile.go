package application

import "strings"

// The CSV helpers below are the Go twin of FinanceTracker.Application.Common.CsvFile:
// a minimal RFC 4180 reader/writer shared by transaction import and export.

// EscapeCSVField quotes a field when it contains a delimiter, a quote or a line
// break, doubling any quote inside it.
func EscapeCSVField(value string) string {
	if !strings.ContainsAny(value, ",\"\n\r") {
		return value
	}
	return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
}

// AppendCSVRow writes one record, terminated by a bare line feed as the .NET
// writer does.
func AppendCSVRow(builder *strings.Builder, fields []string) {
	for i, field := range fields {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(EscapeCSVField(field))
	}
	builder.WriteByte('\n')
}

// ParseCSV splits CSV text into rows of fields, honouring quoted fields that
// span line breaks. A row is emitted only once something has been seen, so
// trailing newlines do not produce an empty record.
func ParseCSV(content string) [][]string {
	state := &csvParser{}
	for index := 0; index < len(content); {
		if state.inQuotes {
			index += state.consumeQuoted(content, index)
			continue
		}
		index += state.consumeUnquoted(content, index)
	}
	state.commitRow()
	return state.rows
}

// csvParser is the port of CsvFile.ParserState; the two consume methods return
// how many bytes they used so a doubled quote can advance by two.
type csvParser struct {
	field    strings.Builder
	row      []string
	touched  bool
	inQuotes bool
	rows     [][]string
}

func (p *csvParser) consumeQuoted(text string, index int) int {
	current := text[index]
	if current != '"' {
		p.field.WriteByte(current)
		return 1
	}
	if index+1 < len(text) && text[index+1] == '"' {
		p.field.WriteByte('"')
		return 2
	}
	p.inQuotes = false
	return 1
}

func (p *csvParser) consumeUnquoted(text string, index int) int {
	switch text[index] {
	case '"':
		p.inQuotes = true
		p.touched = true
	case ',':
		p.row = append(p.row, p.field.String())
		p.field.Reset()
		p.touched = true
	case '\r':
	case '\n':
		p.commitRow()
	default:
		p.field.WriteByte(text[index])
		p.touched = true
	}
	return 1
}

func (p *csvParser) commitRow() {
	if !p.touched && p.field.Len() == 0 && len(p.row) == 0 {
		return
	}

	p.row = append(p.row, p.field.String())
	p.field.Reset()
	p.rows = append(p.rows, p.row)
	p.row = nil
	p.touched = false
}
