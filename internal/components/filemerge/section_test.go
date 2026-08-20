package filemerge

import "testing"

func TestInjectMarkdownSection_AppendNew(t *testing.T) {
	existing := "# My Config\n\nSome user content.\n"
	result := InjectMarkdownSection(existing, "sdd", "SDD workflow enabled.\n")

	want := "# My Config\n\nSome user content.\n\n<!-- cortex-ia:sdd -->\nSDD workflow enabled.\n<!-- /cortex-ia:sdd -->\n"
	if result != want {
		t.Errorf("got:\n%s\nwant:\n%s", result, want)
	}
}

func TestInjectMarkdownSection_ReplaceExisting(t *testing.T) {
	existing := "# Config\n<!-- cortex-ia:sdd -->\nold content\n<!-- /cortex-ia:sdd -->\nuser stuff\n"
	result := InjectMarkdownSection(existing, "sdd", "new content\n")

	want := "# Config\n<!-- cortex-ia:sdd -->\nnew content\n<!-- /cortex-ia:sdd -->\nuser stuff\n"
	if result != want {
		t.Errorf("got:\n%s\nwant:\n%s", result, want)
	}
}

func TestInjectMarkdownSection_RemoveSection(t *testing.T) {
	existing := "# Config\n\n<!-- cortex-ia:sdd -->\nold content\n<!-- /cortex-ia:sdd -->\n"
	result := InjectMarkdownSection(existing, "sdd", "")

	want := "# Config\n"
	if result != want {
		t.Errorf("got:\n%q\nwant:\n%q", result, want)
	}
}

func TestInjectMarkdownSection_EmptyExisting(t *testing.T) {
	result := InjectMarkdownSection("", "cortex", "memory enabled\n")
	want := "<!-- cortex-ia:cortex -->\nmemory enabled\n<!-- /cortex-ia:cortex -->\n"
	if result != want {
		t.Errorf("got:\n%s\nwant:\n%s", result, want)
	}
}

func TestInjectMarkdownSection_EmptyContentNoSection(t *testing.T) {
	existing := "# Config\n"
	result := InjectMarkdownSection(existing, "sdd", "")
	if result != existing {
		t.Errorf("expected unchanged, got:\n%s", result)
	}
}

func TestInjectMarkdownSection_MultipleSections(t *testing.T) {
	existing := ""
	existing = InjectMarkdownSection(existing, "cortex", "memory\n")
	existing = InjectMarkdownSection(existing, "sdd", "workflow\n")

	if got := InjectMarkdownSection(existing, "cortex", "updated memory\n"); got == existing {
		t.Error("expected content to change when updating first section")
	}
}
