package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"regexp"
	"time"
)

var (
	rTags, _ = regexp.Compile(`^[a-z\-0-9]+(,[a-z\-0-9]+)*$`)
)

// Channel holds the schema definition for the Channel entity.
type Channel struct {
	ent.Schema
}

// Fields of the Channel.
func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.Text("channel_name").NotEmpty().Unique(),
		field.Text("display_name").NotEmpty(),
		field.Uint("skip_start").Default(0),
		field.Uint("min_duration"),
		field.Text("url").NotEmpty(),
		field.Bool("is_fav").Default(false),
		field.Bool("is_paused").Default(false),
		field.Bool("is_deleted").Default(false),
		field.Time("created_at").Default(time.Now),
		field.Strings("tags").Optional(),
	}
}

// Edges of the Channel.
func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("recordings", Recording.Type),
		edge.To("jobs", Job.Type),
	}
}
