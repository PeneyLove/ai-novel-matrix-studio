package sandbox

import (
	"testing"
)

func TestManagerSingleton(t *testing.T) {
	ResetForTesting()
	m1 := Manager()
	m2 := Manager()
	if m1 != m2 {
		t.Fatal("Manager() returned different instances")
	}
}

func TestRegisterWriter(t *testing.T) {
	ResetForTesting()
	m := Manager()
	sb := m.RegisterWriter("you are a writer")

	if sb == nil {
		t.Fatal("RegisterWriter returned nil")
	}
	if sb.Role() != "writer" {
		t.Fatalf("role = %q, want writer", sb.Role())
	}
	if m.Writer() != sb {
		t.Fatal("Writer() returned different sandbox")
	}
}

func TestGetOrCreate(t *testing.T) {
	ResetForTesting()
	m := Manager()

	sb1 := m.GetOrCreate("helper", "you help")
	sb2 := m.GetOrCreate("helper", "you help") // same role, same prompt
	if sb1 != sb2 {
		t.Fatal("GetOrCreate should return the same instance for the same role")
	}

	sb3 := m.GetOrCreate("other", "other prompt")
	if sb3 == sb1 {
		t.Fatal("different roles should get different sandboxes")
	}
	if m.Lookup("helper") != sb1 {
		t.Fatal("Lookup lost the helper sandbox")
	}
}

func TestDestroy(t *testing.T) {
	ResetForTesting()
	m := Manager()

	m.GetOrCreate("helper", "prompt")
	m.RegisterWriter("writer prompt")

	if !m.Destroy("helper") {
		t.Fatal("Destroy should succeed for helper")
	}
	if m.Lookup("helper") != nil {
		t.Fatal("helper should be gone after Destroy")
	}
	if m.Destroy("helper") {
		t.Fatal("second Destroy should return false")
	}
	if m.Destroy("writer") {
		t.Fatal("should never destroy writer sandbox")
	}
	if m.Writer() == nil {
		t.Fatal("writer should still exist")
	}
}

func TestDestroyAll(t *testing.T) {
	ResetForTesting()
	m := Manager()
	m.RegisterWriter("writer")
	m.GetOrCreate("a", "a")
	m.GetOrCreate("b", "b")
	m.DestroyAll()

	if m.Writer() == nil {
		t.Fatal("writer should survive DestroyAll")
	}
	if m.Lookup("a") != nil {
		t.Fatal("aux sandbox 'a' should be destroyed")
	}
	if m.Lookup("b") != nil {
		t.Fatal("aux sandbox 'b' should be destroyed")
	}
}

func TestList(t *testing.T) {
	ResetForTesting()
	m := Manager()
	m.RegisterWriter("w")
	m.GetOrCreate("x", "x")

	list := m.List()
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list["writer"] == "" || list["x"] == "" {
		t.Fatalf("list missing entries: %v", list)
	}
}

func TestResetForTesting(t *testing.T) {
	ResetForTesting()
	m1 := Manager()
	m1.RegisterWriter("w")
	m1.GetOrCreate("x", "x")
	if m1.Writer() == nil || m1.Lookup("x") == nil {
		t.Fatal("setup should create sandboxes")
	}

	// Reset should give us a clean state.
	m2 := ResetForTesting()
	if m2.Writer() != nil {
		t.Fatal("after reset, writer should be nil")
	}
	if len(m2.List()) != 0 {
		t.Fatal("after reset, list should be empty")
	}
}
