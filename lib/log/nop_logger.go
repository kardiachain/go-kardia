package log

type nopLogger struct{}

// Interface assertions
var _ Logger = (*nopLogger)(nil)

// NewNopLogger returns a logger that doesn't do anything.
func NewNopLogger() Logger { return &nopLogger{} }

func (nopLogger) Info(string, ...interface{})  {}
func (nopLogger) Debug(string, ...interface{}) {}
func (nopLogger) Error(string, ...interface{}) {}

func (l *nopLogger) With(...interface{}) Logger {
	return l
}

func (l *nopLogger) New(ctx ...interface{}) Logger     { return l }
func (nopLogger) AddTag(tag string)                    {}
func (nopLogger) GetHandler() Handler                  { return nil }
func (nopLogger) SetHandler(h Handler)                 {}
func (nopLogger) Trace(msg string, ctx ...interface{}) {}
func (nopLogger) Warn(msg string, ctx ...interface{})  {}
func (nopLogger) Crit(msg string, ctx ...interface{})  {}
