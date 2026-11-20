// 9RouterX — derived from the work of https://github.com/yanstd888/ — port and modifications by the maintainers — https://t.me/aiclub_id

package models

import (
	"reflect"
	"testing"
)

// The dashboard grouping, combo cross-family warnings, and dispatch family
// logging all read ModelFamily/ComboFamilies — these tests pin the collapse
// rules the three consumers share.

func TestModelFamily(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"GPT-4.1", "gpt"},
		{"gpt-4.1-mini", "gpt-mini"},
		{"claude-sonnet-4", "claude-sonnet"},
		{"gemini-2.5-pro:free", "gemini-pro"},
		{"gemini-3.1-flash-preview", "gemini-flash"},
		{"o3-mini", "o3-mini"},
		{"kimi-k2.6", "kimi"},
		{"1.0", "1.0"},
	} {
		if got := ModelFamily(tc.id); got != tc.want {
			t.Errorf("ModelFamily(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}

func TestComboFamilies(t *testing.T) {
	families, cross := ComboFamilies([]string{
		"openai/gpt-4.1",
		"openai/gpt-4.1-mini",
		"anthropic/claude-sonnet-4",
	})
	want := []string{"gpt", "gpt-mini", "claude-sonnet"}
	if !reflect.DeepEqual(families, want) {
		t.Errorf("families = %v, want %v", families, want)
	}
	if !cross {
		t.Error("cross = false, want true (gpt line vs claude line)")
	}
}

func TestComboFamiliesIgnoresUnresolvableEntries(t *testing.T) {
	for _, members := range [][]string{
		{"nested-combo-name"},
		{"x/"},
		{""},
	} {
		families, cross := ComboFamilies(members)
		if len(families) != 0 || cross {
			t.Errorf("ComboFamilies(%v) = %v, %v; want no families and no crossing", members, families, cross)
		}
	}
}

func TestComboFamiliesSameLineIsNotCross(t *testing.T) {
	_, cross := ComboFamilies([]string{"openai/gpt-4.1", "openai/gpt-4.1-mini"})
	if cross {
		t.Error("cross = true for a single gpt-line ladder; want false")
	}
}
