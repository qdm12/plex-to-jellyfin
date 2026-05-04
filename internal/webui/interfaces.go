package webui

type Logger interface {
	Debugf(format string, args ...any)
	Info(message string)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}
