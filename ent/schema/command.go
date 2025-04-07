package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Command holds the schema definition for the Command entity.
type Command struct {
	ent.Schema
}

// Fields of the Command.
func (Command) Fields() []ent.Field {
	return []ent.Field{
		field.Int("pid").Optional(),
		field.Text("command"),
		field.Text("progress").Optional(),
		field.Text("info").Optional(),
		field.Text("args").Optional(),
	}
}

// Edges of the Command.
func (Command) Edges() []ent.Edge {
	return nil
}
