package command

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/RenlySir/timongo/internal/mongoerrors"
)

// Handler processes one MongoDB command document.
type Handler interface {
	Handle(ctx context.Context, cmd bson.M) (bson.M, error)
}

// HandlerFunc adapts a function into a Handler.
type HandlerFunc func(ctx context.Context, cmd bson.M) (bson.M, error)

// Handle processes one command document.
func (f HandlerFunc) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	return f(ctx, cmd)
}

// Registry dispatches command documents by command name.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds or replaces a command handler.
func (r *Registry) Register(name string, h Handler) {
	r.handlers[name] = h
}

// Handle dispatches a command to a registered handler.
func (r *Registry) Handle(ctx context.Context, cmd bson.M) (bson.M, error) {
	name, _, err := Name(cmd)
	if err != nil {
		return nil, err
	}
	h, ok := r.handlers[name]
	if !ok {
		return nil, mongoerrors.New(mongoerrors.CodeCommandNotFound, "CommandNotFound", "no such command: %s", name)
	}
	return h.Handle(ctx, cmd)
}

var knownNames = []string{
	"hello",
	"isMaster",
	"ismaster",
	"ping",
	"buildInfo",
	"insert",
	"find",
}

// Name returns the MongoDB command name.
func Name(cmd bson.M) (string, any, error) {
	for _, key := range knownNames {
		if value, ok := cmd[key]; ok {
			return key, value, nil
		}
	}
	for key, value := range cmd {
		if len(key) > 0 && key[0] != '$' {
			return key, value, nil
		}
	}
	return "", nil, mongoerrors.New(mongoerrors.CodeBadValue, "BadValue", "empty command")
}
