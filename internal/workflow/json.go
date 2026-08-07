package workflow

import (
	"encoding/json"
	"time"
)

// workflowDTO is the JSON wire shape for Workflow. Workflow's own
// fields are unexported (so nothing outside this package can construct
// an invalid one directly), which means the standard encoding/json
// reflection-based (un)marshaling can't see them — hence these
// explicit MarshalJSON/UnmarshalJSON methods rather than relying on
// struct tags on Workflow itself.
type workflowDTO struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     int               `json:"version"`
	Description string            `json:"description,omitempty"`
	Steps       []Step            `json:"steps"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// MarshalJSON implements json.Marshaler.
func (w Workflow) MarshalJSON() ([]byte, error) {
	return json.Marshal(workflowDTO{
		ID:          w.id,
		Name:        w.name,
		Version:     w.version,
		Description: w.description,
		Steps:       w.steps,
		Metadata:    w.metadata,
		CreatedAt:   w.createdAt,
	})
}

// UnmarshalJSON implements json.Unmarshaler. It routes through New, so
// a Workflow parsed from JSON (a workflow definition file, or a future
// API request body) gets exactly the same validation — unique step
// IDs, resolvable dependencies, acyclic graph — as one built directly
// in Go. There is no way to unmarshal your way to an invalid Workflow.
//
// Note this always stamps CreatedAt to time.Now() via New, same as any
// other construction path — it does not preserve a `created_at` value
// from the input JSON. Restoring a Workflow's original creation time
// from persisted storage is a Milestone 7 concern for the Postgres
// repository, which will need its own reconstitution path rather than
// going through UnmarshalJSON.
func (w *Workflow) UnmarshalJSON(data []byte) error {
	var dto workflowDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return err
	}

	var opts []Option
	if dto.Description != "" {
		opts = append(opts, WithDescription(dto.Description))
	}
	if len(dto.Metadata) > 0 {
		opts = append(opts, WithMetadata(dto.Metadata))
	}

	built, err := New(dto.ID, dto.Name, dto.Version, dto.Steps, opts...)
	if err != nil {
		return err
	}
	*w = built
	return nil
}
