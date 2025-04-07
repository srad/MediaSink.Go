package schema

import "entgo.io/ent"

// Setting holds the schema definition for the Setting entity.
type Setting struct {
	ent.Schema
}

// Fields of the Setting.
func (Setting) Fields() []ent.Field {
	return nil
}

// Edges of the Setting.
func (Setting) Edges() []ent.Edge {
	return nil
}
