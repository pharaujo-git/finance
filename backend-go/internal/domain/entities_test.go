package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pharaujo/finance/backend-go/internal/domain"
)

// separator is the unit separator the TagsRaw column packs tags with.
const separator = string(rune(31))

func TestJoinTagsTrimsAndDropsBlanks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		tags []string
		want string
	}{
		{"nothing", nil, ""},
		{"one", []string{"reading"}, "reading"},
		{"trimmed", []string{"  reading  "}, "reading"},
		{"blanks dropped", []string{"a", "", "   ", "b"}, "a" + separator + "b"},
		{"all blank", []string{"", "  "}, ""},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := domain.JoinTags(testCase.tags); got != testCase.want {
				t.Errorf("JoinTags(%q) = %q, want %q", testCase.tags, got, testCase.want)
			}
		})
	}
}

func TestSplitTagsNeverReturnsNil(t *testing.T) {
	t.Parallel()

	empty := domain.SplitTags("")
	if empty == nil {
		t.Fatal("SplitTags(\"\") is nil, which would render as null instead of []")
	}

	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(encoded) != "[]" {
		t.Errorf("Marshal = %s, want []", encoded)
	}

	tags := domain.SplitTags("a" + separator + separator + "b")
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Errorf("SplitTags = %q, want [a b] with the empty entry removed", tags)
	}
}

func TestTagsRoundTrip(t *testing.T) {
	t.Parallel()

	raw := domain.JoinTags([]string{" morning ", "treat"})
	if !strings.Contains(raw, separator) {
		t.Fatalf("raw = %q, want the unit separator between tags", raw)
	}

	tags := domain.SplitTags(raw)
	if len(tags) != 2 || tags[0] != "morning" || tags[1] != "treat" {
		t.Errorf("round trip = %q", tags)
	}
}

func TestTagSeparatorIsTheUnitSeparator(t *testing.T) {
	t.Parallel()

	if domain.TagSeparator != 31 {
		t.Errorf("TagSeparator = %d, want 31 (U+001F)", domain.TagSeparator)
	}
}
