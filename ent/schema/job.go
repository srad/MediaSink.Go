package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/google/uuid" // For default UUIDs
)

// Job holds the schema definition for the Job entity.
type Job struct {
	ent.Schema
}

// Fields of the Job.
func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New).
			Immutable(),
		field.String("type").
			NotEmpty(),
		field.Bytes("payload").
			NotEmpty(),
		field.Enum("status").
			Values("pending", "running", "failed", "done").
			Default("pending"),
		field.Int("attempt_count").
			Default(0).
			NonNegative(),
		field.String("last_error").
			Optional().
			Nillable(),
		field.String("details").
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now), // Automatically update on modification
	}
}

// Edges of the Job.
func (Job) Edges() []ent.Edge {
	// No edges defined for this simple job queue
	return nil
}

// Indexes of the Job.
func (Job) Indexes() []ent.Index {
	return []ent.Index{
		// Index for efficiently finding pending jobs ordered by creation time
		index.Fields("status", "created_at"),
	}
}
