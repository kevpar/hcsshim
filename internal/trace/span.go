package trace

import (
	"context"

	"github.com/Microsoft/CorrelationVector-Go/correlationvector"
	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/sirupsen/logrus"
)

// Field represents a single field that can be logged as part of an event.
type Field struct {
	Name  string
	Value interface{}
}

type Level uint8

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

// Span represents an operation over a period of time. It can be associated with
// a parent span, and point-in-time events can be logged in the context of the
// span.
type Span struct {
	name     string
	spanID   *guid.GUID
	parentID *guid.GUID
	cv       *correlationvector.CorrelationVector
	baggage  []Field
	level Level
}

func deriveCV(cv *correlationvector.CorrelationVector) *correlationvector.CorrelationVector {
	var newCV *correlationvector.CorrelationVector
	var err error

	if cv == nil {
		newCV, err = correlationvector.NewCorrelationVectorWithVersion(correlationvector.V2Version)
	} else {
		newCV, err = correlationvector.Extend(cv.Increment())
	}

	if err != nil {
		newCV = nil
	}
	return newCV
}

// NewSpan returns a new span as a child of this span, and writes the span start
// event.
func (s *Span) NewSpan(ctx context.Context, name string, level Level, baggage []Field) (context.Context, *Span) {
	g, err := guid.NewV4()
	if err != nil {
		g = &guid.GUID{}
	}
	child := &Span{
		name:     name,
		spanID:   g,
		parentID: s.spanID,
		baggage:  append(s.baggage, baggage...),
		cv:       deriveCV(s.cv),
		level: level,
	}
	child.start()
	return context.WithValue(ctx, spanKey, child), child
}

func (s *Span) start() {
	s.log("SpanStart", etw.LevelInfo, etw.OpcodeStart, logrus.InfoLevel, []Field{{"Name", s.name}})
}

// End terminates this span, and writes the span stop event.
func (s *Span) End(err error) {
	etwLevel := etw.LevelInfo
	logrusLevel := logrus.InfoLevel
	if err != nil {
		etwLevel = etw.LevelError
		logrusLevel = logrus.ErrorLevel
	}
	s.log("SpanEnd", etwLevel, etw.OpcodeStop, logrusLevel, []Field{{"Name", s.name}, {"Error", err}})
}

// Error logs an error level event associated with this span.
func (s *Span) Error(name string, fields []Field) {
	s.log(name, etw.LevelError, etw.OpcodeInfo, logrus.ErrorLevel, fields)
}

// Warn logs an warn level event associated with this span.
func (s *Span) Warn(name string, fields []Field) {
	s.log(name, etw.LevelWarning, etw.OpcodeInfo, logrus.WarnLevel, fields)
}

// Info logs an info level event associated with this span.
func (s *Span) Info(name string, fields []Field) {
	s.log(name, etw.LevelInfo, etw.OpcodeInfo, logrus.InfoLevel, fields)
}

// Debug logs an debug level event associated with this span.
func (s *Span) Debug(name string, fields []Field) {
	s.log(name, etw.LevelVerbose, etw.OpcodeInfo, logrus.DebugLevel, fields)
}

func (s *Span) log(name string, etwLevel etw.Level, etwOpcode etw.Opcode, logrusLevel logrus.Level, fields []Field) {
	s.logETW(name, etwLevel, etwOpcode, fieldsToETW(fields))
	s.logLogrus(name, logrusLevel, fieldsToLogrus(fields))
}

func (s *Span) logETW(name string, level etw.Level, opcode etw.Opcode, fields []etw.FieldOpt) {
	allFields := make([]etw.FieldOpt, 0, len(fields)+1)
	allFields = append(allFields, etw.StringField("CV", s.cvValue()))
	allFields = append(allFields, etw.Struct("Baggage", fieldsToETW(s.baggage)...))
	allFields = append(allFields, fields...)
	p.WriteEvent(
		name,
		etw.WithEventOpts(
			etw.WithLevel(level),
			etw.WithOpcode(opcode),
			etw.WithActivityID(s.spanID),
			etw.WithRelatedActivityID(s.parentID),
		),
		etw.WithFields(
			allFields...,
		),
	)
}

func (s *Span) logLogrus(name string, level logrus.Level, fields logrus.Fields) {
	fields["CV"] = s.cvValue()
	fields["Baggage"] = fieldsToLogrus(s.baggage)
	logrus.WithFields(fields).Log(level, name)
}

func (s *Span) cvValue() string {
	if s.cv != nil {
		return s.cv.Value()
	}
	return ""
}

func fieldsToETW(fields []Field) []etw.FieldOpt {
	etwFields := make([]etw.FieldOpt, 0, len(fields))
	for _, field := range fields {
		etwFields = append(etwFields, etw.SmartField(field.Name, field.Value))
	}
	return etwFields
}

func levelToETW(level Level) etw.Level {
	switch level {
	case LevelError:
		return etw.LevelError
	case LevelWarn:
		return etw.LevelWarning
	case LevelInfo:
		return etw.LevelInfo
	case LevelDebug:
		return etw.LevelVerbose
	}
}

func fieldsToLogrus(fields []Field) logrus.Fields {
	logrusFields := logrus.Fields{}
	for _, field := range fields {
		logrusFields[field.Name] = field.Value
	}
	return logrusFields
}
