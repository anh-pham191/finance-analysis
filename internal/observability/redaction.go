package observability

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
)

var (
	secretKeyPattern       = regexp.MustCompile(`(?i)(token|authorization|password|secret|api[_-]?key)`)
	bearerPattern          = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
	tokenValuePattern      = regexp.MustCompile(`(?i)(["']?\b(?:app_token|user_token)["']?\s*[:=]\s*["']?)[^"',}\s]+(["']?)`)
	standaloneTokenPattern = regexp.MustCompile(`(?i)\b(?:app_token|user_token)_[A-Za-z0-9_=-]{8,}\b`)
	base64LikePattern      = regexp.MustCompile(`\b[A-Za-z0-9+/=]{32,}\b`)
)

type redactingHandler struct {
	next slog.Handler
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return redactingHandler{next: next}
}

func (h redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h redactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, RedactString(record.Message), record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactAttr(attr))
	}
	return redactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h redactingHandler) WithGroup(name string) slog.Handler {
	return redactingHandler{next: h.next.WithGroup(name)}
}

func RedactString(value string) string {
	value = bearerPattern.ReplaceAllString(value, "Bearer ***")
	value = tokenValuePattern.ReplaceAllString(value, "${1}***${2}")
	value = standaloneTokenPattern.ReplaceAllString(value, "***")
	return base64LikePattern.ReplaceAllString(value, "***")
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if secretKeyPattern.MatchString(attr.Key) {
		attr.Value = slog.StringValue("***")
		return attr
	}

	switch attr.Value.Kind() {
	case slog.KindString:
		attr.Value = slog.StringValue(RedactString(attr.Value.String()))
	case slog.KindAny:
		switch value := attr.Value.Any().(type) {
		case error:
			attr.Value = slog.StringValue(RedactString(value.Error()))
		case fmt.Stringer:
			attr.Value = slog.StringValue(RedactString(value.String()))
		}
	case slog.KindGroup:
		group := attr.Value.Group()
		redacted := make([]slog.Attr, 0, len(group))
		for _, groupAttr := range group {
			redacted = append(redacted, redactAttr(groupAttr))
		}
		attr.Value = slog.GroupValue(redacted...)
	}

	return attr
}
