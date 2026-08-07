package policy

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidTimeWindow is returned by NewTimeWindowRule for an
// out-of-range hour.
var ErrInvalidTimeWindow = errors.New("policy: time window hours must be 0-23")

// TimeWindowRule denies when Input.Now (in UTC) falls outside
// [startHourUTC, endHourUTC). A window where end <= start is treated
// as wrapping past midnight (e.g. 22 -> 6 means "allowed overnight,
// 22:00 through 05:59"). It has no opinion when Input.Now is the zero
// value, since that means the caller isn't supplying a time-relevant
// decision point at all.
type TimeWindowRule struct {
	startHourUTC, endHourUTC int
}

// NewTimeWindowRule builds a TimeWindowRule for the half-open interval
// [startHourUTC, endHourUTC), both in 0-23.
func NewTimeWindowRule(startHourUTC, endHourUTC int) (TimeWindowRule, error) {
	if startHourUTC < 0 || startHourUTC > 23 || endHourUTC < 0 || endHourUTC > 23 {
		return TimeWindowRule{}, fmt.Errorf("%w: got start=%d end=%d", ErrInvalidTimeWindow, startHourUTC, endHourUTC)
	}
	return TimeWindowRule{startHourUTC: startHourUTC, endHourUTC: endHourUTC}, nil
}

// Name implements Rule.
func (TimeWindowRule) Name() string { return "time-window" }

// Evaluate implements Rule.
func (r TimeWindowRule) Evaluate(_ context.Context, in Input) (Decision, bool, error) {
	if in.Now.IsZero() {
		return Decision{}, false, nil
	}

	hour := in.Now.UTC().Hour()
	var inWindow bool
	if r.startHourUTC < r.endHourUTC {
		inWindow = hour >= r.startHourUTC && hour < r.endHourUTC
	} else {
		inWindow = hour >= r.startHourUTC || hour < r.endHourUTC
	}

	if inWindow {
		return Decision{}, false, nil
	}
	return Decision{
		Effect: EffectDeny,
		Reason: fmt.Sprintf("outside allowed hours [%02d:00-%02d:00 UTC)", r.startHourUTC, r.endHourUTC),
	}, true, nil
}
