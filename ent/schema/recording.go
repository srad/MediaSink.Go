package schema

import "entgo.io/ent"

// Recording holds the schema definition for the Recording entity.
type Recording struct {
	ent.Schema
}

// Fields of the Recording.
func (Recording) Fields() []ent.Field {
	return nil
}

// Edges of the Recording.
func (Recording) Edges() []ent.Edge {
	return nil
}
