package agentbuilder

import (
	"strings"
	"testing"
)

const validSKILL = `# My Custom Reviewer

## Description
Reviews PRs for cortex-ia conventions.

## Trigger
When user asks to "review the PR" or "ml review".

## Instructions
1. Read the diff.
2. Compare against project standards.
3. Report findings as a checklist.
`

func TestParse_Valid(t *testing.T) {
	got, err := Parse(validSKILL)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Name != "my-custom-reviewer" {
		t.Errorf("Name = %q", got.Name)
	}
	if got.Title != "My Custom Reviewer" {
		t.Errorf("Title = %q", got.Title)
	}
	if !strings.Contains(got.Description, "Reviews PRs") {
		t.Errorf("Description = %q", got.Description)
	}
	if !strings.Contains(got.Trigger, "review the PR") {
		t.Errorf("Trigger = %q", got.Trigger)
	}
}

func TestParse_StripsCodeFences(t *testing.T) {
	wrapped := "```markdown\n" + validSKILL + "\n```\n"
	got, err := Parse(wrapped)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if strings.Contains(got.Content, "```") {
		t.Errorf("code fences not stripped: %q", got.Content)
	}
}

func TestParse_MissingTitle(t *testing.T) {
	noTitle := strings.Replace(validSKILL, "# My Custom Reviewer", "Review skill", 1)
	if _, err := Parse(noTitle); err == nil {
		t.Fatal("expected error for missing H1")
	}
}

func TestParse_MissingSection(t *testing.T) {
	noTrigger := strings.Replace(validSKILL, "## Trigger\nWhen user asks to \"review the PR\" or \"ml review\".\n\n", "", 1)
	if _, err := Parse(noTrigger); err == nil {
		t.Fatal("expected error for missing trigger")
	}
}

func TestParse_EmptySection(t *testing.T) {
	emptyDesc := strings.Replace(validSKILL, "Reviews PRs for cortex-ia conventions.", "", 1)
	if _, err := Parse(emptyDesc); err == nil {
		t.Fatal("expected error for empty description")
	}
}

func TestTitleToName(t *testing.T) {
	cases := map[string]string{
		"My Custom Agent":  "my-custom-agent",
		"v2.0 Reviewer!!!": "v2-0-reviewer",
		"   spaces   ":     "spaces",
		"---":              "",
	}
	for in, want := range cases {
		if got := titleToName(in); got != want {
			t.Errorf("titleToName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParse_EmptyAfterStrip(t *testing.T) {
	if _, err := Parse("```\n```"); err == nil {
		t.Fatal("expected error for empty content")
	}
}
