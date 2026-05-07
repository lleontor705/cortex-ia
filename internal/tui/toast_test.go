package tui

import "testing"

func TestToastQueue_Push(t *testing.T) {
	var q ToastQueue
	q.Push(Toast{Text: "first", Visible: true})
	if len(q.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(q.Items))
	}
	q.Push(Toast{Text: "second", Visible: true})
	q.Push(Toast{Text: "third", Visible: true})
	if len(q.Items) != 3 {
		t.Errorf("len(Items) = %d, want 3", len(q.Items))
	}
}

func TestToastQueue_PushEvictsOldest(t *testing.T) {
	var q ToastQueue
	q.Push(Toast{Text: "first"})
	q.Push(Toast{Text: "second"})
	q.Push(Toast{Text: "third"})
	q.Push(Toast{Text: "fourth"}) // should evict "first"

	if len(q.Items) != maxToasts {
		t.Errorf("len(Items) = %d, want %d", len(q.Items), maxToasts)
	}
	if q.Items[0].Text != "second" {
		t.Errorf("oldest after eviction = %q, want %q", q.Items[0].Text, "second")
	}
	if q.Items[len(q.Items)-1].Text != "fourth" {
		t.Errorf("newest = %q, want %q", q.Items[len(q.Items)-1].Text, "fourth")
	}
}

func TestToastQueue_Dismiss(t *testing.T) {
	var q ToastQueue
	q.Push(Toast{Text: "a"})
	q.Push(Toast{Text: "b"})

	q.Dismiss()
	if len(q.Items) != 1 {
		t.Errorf("len(Items) after Dismiss = %d, want 1", len(q.Items))
	}
	if q.Items[0].Text != "b" {
		t.Errorf("remaining item = %q, want %q", q.Items[0].Text, "b")
	}
}

func TestToastQueue_DismissEmpty(t *testing.T) {
	var q ToastQueue
	q.Dismiss() // should not panic
	if len(q.Items) != 0 {
		t.Error("empty queue should remain empty after Dismiss")
	}
}

func TestToastQueue_HasVisible(t *testing.T) {
	var q ToastQueue
	if q.HasVisible() {
		t.Error("empty queue should not have visible toasts")
	}
	q.Push(Toast{Text: "a"})
	if !q.HasVisible() {
		t.Error("queue with items should have visible toasts")
	}
}

func TestRenderToastQueue_Empty(t *testing.T) {
	var q ToastQueue
	out := renderToastQueue(q, 80)
	if out != "" {
		t.Errorf("empty queue should render empty string, got %q", out)
	}
}

func TestRenderToastQueue_WithItems(t *testing.T) {
	var q ToastQueue
	q.Push(Toast{Text: "success message"})
	q.Push(Toast{Text: "error message", IsError: true})

	out := renderToastQueue(q, 80)
	if out == "" {
		t.Error("queue with items should render non-empty")
	}
}

func TestRenderToast_NotVisible(t *testing.T) {
	t1 := Toast{Text: "hidden", Visible: false}
	if renderToast(t1, 80) != "" {
		t.Error("invisible toast should render empty")
	}
}

func TestRenderToast_Visible(t *testing.T) {
	t1 := Toast{Text: "shown", Visible: true}
	if renderToast(t1, 80) == "" {
		t.Error("visible toast should render non-empty")
	}
}

func TestRenderToast_Error(t *testing.T) {
	t1 := Toast{Text: "fail", IsError: true, Visible: true}
	out := renderToast(t1, 80)
	if out == "" {
		t.Error("error toast should render")
	}
}
