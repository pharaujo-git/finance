using System.Text;

namespace FinanceTracker.Application.Common;

/// <summary>Minimal RFC 4180 reader/writer used by transaction import and export.</summary>
public static class CsvFile
{
    /// <summary>Quotes a single field when it contains a delimiter, quote or line break.</summary>
    public static string EscapeField(string? value)
    {
        var text = value ?? string.Empty;
        var needsQuotes = text.AsSpan().IndexOfAny(',', '"', '\n') >= 0
                          || text.Contains('\r', StringComparison.Ordinal);
        return needsQuotes
            ? string.Concat("\"", text.Replace("\"", "\"\"", StringComparison.Ordinal), "\"")
            : text;
    }

    public static void AppendRow(StringBuilder builder, IEnumerable<string?> fields)
    {
        ArgumentNullException.ThrowIfNull(builder);
        builder.AppendJoin(',', fields.Select(EscapeField)).Append('\n');
    }

    /// <summary>Splits CSV text into rows of fields, honouring quoted fields that span line breaks.</summary>
    public static List<List<string>> Parse(string? content)
    {
        var state = new ParserState();
        var text = content ?? string.Empty;
        var index = 0;

        while (index < text.Length)
        {
            index += state.InQuotes
                ? state.ConsumeQuoted(text, index)
                : state.ConsumeUnquoted(text, index);
        }

        state.CommitRow();
        return state.Rows;
    }

    private sealed class ParserState
    {
        private readonly StringBuilder _field = new();
        private List<string> _row = [];
        private bool _touched;

        public List<List<string>> Rows { get; } = [];

        public bool InQuotes { get; private set; }

        /// <summary>Consumes one character inside a quoted field; returns how many characters were used.</summary>
        public int ConsumeQuoted(string text, int index)
        {
            var current = text[index];
            if (current != '"')
            {
                _field.Append(current);
                return 1;
            }

            if (index + 1 < text.Length && text[index + 1] == '"')
            {
                _field.Append('"');
                return 2;
            }

            InQuotes = false;
            return 1;
        }

        /// <summary>Consumes one character outside a quoted field; returns how many characters were used.</summary>
        public int ConsumeUnquoted(string text, int index)
        {
            switch (text[index])
            {
                case '"':
                    InQuotes = true;
                    _touched = true;
                    break;
                case ',':
                    _row.Add(_field.ToString());
                    _field.Clear();
                    _touched = true;
                    break;
                case '\r':
                    break;
                case '\n':
                    CommitRow();
                    break;
                default:
                    _field.Append(text[index]);
                    _touched = true;
                    break;
            }

            return 1;
        }

        public void CommitRow()
        {
            if (!_touched && _field.Length == 0 && _row.Count == 0)
            {
                return;
            }

            _row.Add(_field.ToString());
            _field.Clear();
            Rows.Add(_row);
            _row = [];
            _touched = false;
        }
    }
}
