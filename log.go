package selector

import (
	"fmt"
	"os"
)

type Logger interface {
	Errorf(format string, args ...any)
	Infof(format string, args ...any)
}

type defaultLogger struct {
}

func (l *defaultLogger) Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func (l *defaultLogger) Infof(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format, args...)
}
