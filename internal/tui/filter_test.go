package tui

import "testing"

func TestNewFilterInput(t *testing.T) {
	f := NewFilterInput()
	if f.Active {
		t.Error("new filter should not be active")
	}
	if f.Query() != "" {
		t.Error("new filter should have empty query")
	}
}

func TestFilterInput_Toggle(t *testing.T) {
	f := NewFilterInput()

	f.Toggle()
	if !f.Active {
		t.Error("after toggle, filter should be active")
	}

	f.Toggle()
	if f.Active {
		t.Error("after second toggle, filter should be inactive")
	}
}

func TestFilterInput_Activate(t *testing.T) {
	f := NewFilterInput()
	f.Activate()
	if !f.Active {
		t.Error("after activate, filter should be active")
	}
}

func TestFilterInput_Deactivate_ClearsValue(t *testing.T) {
	f := NewFilterInput()
	f.Activate()
	f.Input.SetValue("test")
	f.Deactivate()

	if f.Active {
		t.Error("after deactivate, filter should be inactive")
	}
	if f.Input.Value() != "" {
		t.Errorf("after deactivate, value should be empty, got %q", f.Input.Value())
	}
}

func TestFilterInput_Matches(t *testing.T) {
	f := NewFilterInput()
	f.Activate()

	// Empty query matches everything
	if !f.Matches("anything") {
		t.Error("empty query should match anything")
	}

	// Set query
	f.Input.SetValue("hello")
	if !f.Matches("say hello world") {
		t.Error("should match substring")
	}
	if f.Matches("goodbye") {
		t.Error("should not match unrelated string")
	}

	// Case insensitive
	f.Input.SetValue("HELLO")
	if !f.Matches("hello world") {
		t.Error("should match case-insensitively")
	}
}

func TestFilterInput_Query(t *testing.T) {
	f := NewFilterInput()
	f.Activate()
	f.Input.SetValue("FooBar")
	if f.Query() != "foobar" {
		t.Errorf("Query() = %q, want %q", f.Query(), "foobar")
	}
}

func TestFilterInput_View_InactiveReturnsEmpty(t *testing.T) {
	f := NewFilterInput()
	if f.View() != "" {
		t.Error("inactive filter should return empty view")
	}
}

func TestFilterInput_View_ActiveReturnsContent(t *testing.T) {
	f := NewFilterInput()
	f.Activate()
	v := f.View()
	if v == "" {
		t.Error("active filter should return non-empty view")
	}
}
