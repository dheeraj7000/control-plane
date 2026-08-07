package id_test

import (
	"strings"
	"testing"

	"github.com/dheeraj7000/control-plane/pkg/id"
)

func TestNew_HasPrefix(t *testing.T) {
	got := id.New("exec")
	if !strings.HasPrefix(got, "exec_") {
		t.Errorf("New(%q) = %q, want prefix %q", "exec", got, "exec_")
	}
}

func TestNew_Unique(t *testing.T) {
	a := id.New("wf")
	b := id.New("wf")
	if a == b {
		t.Errorf("New() returned the same id twice: %q", a)
	}
}
