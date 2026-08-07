package policy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dheeraj7000/control-plane/internal/policy"
)

func TestNewTimeWindowRule_InvalidHours(t *testing.T) {
	if _, err := policy.NewTimeWindowRule(-1, 5); !errors.Is(err, policy.ErrInvalidTimeWindow) {
		t.Fatalf("NewTimeWindowRule() error = %v, want ErrInvalidTimeWindow", err)
	}
	if _, err := policy.NewTimeWindowRule(0, 24); !errors.Is(err, policy.ErrInvalidTimeWindow) {
		t.Fatalf("NewTimeWindowRule() error = %v, want ErrInvalidTimeWindow", err)
	}
}

func at(hour int) time.Time {
	return time.Date(2026, 8, 7, hour, 0, 0, 0, time.UTC)
}

func TestTimeWindowRule_WithinWindow(t *testing.T) {
	r, err := policy.NewTimeWindowRule(9, 17)
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}
	_, ok, err := r.Evaluate(context.Background(), policy.Input{Now: at(12)})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if ok {
		t.Fatal("Evaluate() ok = true within the allowed window, want false (no opinion)")
	}
}

func TestTimeWindowRule_OutsideWindow(t *testing.T) {
	r, err := policy.NewTimeWindowRule(9, 17)
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}
	d, ok, err := r.Evaluate(context.Background(), policy.Input{Now: at(20)})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || d.Allowed() {
		t.Fatalf("Evaluate() = (%+v, %v), want denied outside the window", d, ok)
	}
}

func TestTimeWindowRule_WrapsPastMidnight(t *testing.T) {
	r, err := policy.NewTimeWindowRule(22, 6) // allowed overnight
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}

	for _, hour := range []int{23, 0, 5} {
		_, ok, err := r.Evaluate(context.Background(), policy.Input{Now: at(hour)})
		if err != nil {
			t.Fatalf("Evaluate() returned error: %v", err)
		}
		if ok {
			t.Errorf("hour %d: ok = true, want false (within overnight window)", hour)
		}
	}

	d, ok, err := r.Evaluate(context.Background(), policy.Input{Now: at(12)})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if !ok || d.Allowed() {
		t.Fatalf("hour 12: Evaluate() = (%+v, %v), want denied (outside overnight window)", d, ok)
	}
}

func TestTimeWindowRule_NoOpinionWhenNowZero(t *testing.T) {
	r, err := policy.NewTimeWindowRule(9, 17)
	if err != nil {
		t.Fatalf("NewTimeWindowRule() returned error: %v", err)
	}
	_, ok, err := r.Evaluate(context.Background(), policy.Input{})
	if err != nil {
		t.Fatalf("Evaluate() returned error: %v", err)
	}
	if ok {
		t.Fatal("Evaluate() ok = true with zero-value Now, want false")
	}
}
