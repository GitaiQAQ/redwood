package log

import (
	"fmt"
	"strings"

	"github.com/brynbellomy/klog"
)

// Logger abstracts basic logging functions.
type Logger interface {
	SetLogLabel(inLabel string)
	GetLogLabel() string
	GetLogPrefix() string
	Debug(args ...interface{})
	Debugf(inFormat string, args ...interface{})
	Debugw(inFormat string, fields Fields)
	Success(args ...interface{})
	Successf(inFormat string, args ...interface{})
	Successw(inFormat string, fields Fields)
	LogV(inVerboseLevel int32) bool
	Info(inVerboseLevel int32, args ...interface{})
	Infof(inVerboseLevel int32, inFormat string, args ...interface{})
	Infow(inFormat string, fields Fields)
	Warn(args ...interface{})
	Warnf(inFormat string, args ...interface{})
	Warnw(inFormat string, fields Fields)
	Error(args ...interface{})
	Errorf(inFormat string, args ...interface{})
	Errorw(inFormat string, fields Fields)
	Fatalf(inFormat string, args ...interface{})
}

type logger struct {
	hasPrefix bool
	logPrefix string
	logLabel  string
}

var longestLabel int

// NewLogger creates and inits a new Logger with the given label.
func NewLogger(label string) Logger {
	l := &logger{}
	if label != "" {
		l.SetLogLabel(label)
	}
	return l
}

// Fatalf -- see Fatalf (above)
func Fatalf(inFormat string, args ...interface{}) {
	gLogger.Fatalf(inFormat, args...)
}

var gLogger = logger{}

// SetLogLabel sets the label prefix for all entries logged.
func (l *logger) SetLogLabel(inLabel string) {
	l.logLabel = inLabel
	l.hasPrefix = len(inLabel) > 0
	if l.hasPrefix {
		l.logPrefix = fmt.Sprintf("[%s] ", inLabel)
		if len(l.logPrefix) > longestLabel {
			longestLabel = len(l.logPrefix)
		}
	}
}

// GetLogLabel returns the label last set via SetLogLabel()
func (l *logger) GetLogLabel() string {
	return l.logLabel
}

// GetLogPrefix returns the the text that prefixes all log messages for this context.
func (l *logger) GetLogPrefix() string {
	return l.logPrefix
}

// LogV returns true if logging is currently enabled for log verbose level.
func (l *logger) LogV(inVerboseLevel int32) bool {
	return bool(klog.V(klog.Level(inVerboseLevel)))
}

func (l *logger) Debug(args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.DebugDepth(1, args...)
	}
}

func (l *logger) Debugf(inFormat string, args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.DebugDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Debugw(msg string, fields Fields) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.DebugDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fields))
	} else {
		klog.DebugDepth(1, fmt.Sprintf(msg+" %v", fields))
	}
}

func (l *logger) Success(args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.SuccessDepth(1, args...)
	}
}

func (l *logger) Successf(inFormat string, args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.SuccessDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Successw(msg string, fields Fields) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.SuccessDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fields))
	} else {
		klog.SuccessDepth(1, fmt.Sprintf(msg+" %v", fields))
	}
}

// Info logs to the INFO log.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Verbose level conventions:
//   0. Enabled during production and field deployment.  Use this for important high-level info.
//   1. Enabled during testing and development. Use for high-level changes in state, mode, or connection.
//   2. Enabled during low-level debugging and troubleshooting.
func (l *logger) Info(inVerboseLevel int32, args ...interface{}) {
	logIt := true
	if inVerboseLevel > 0 {
		logIt = bool(klog.V(klog.Level(inVerboseLevel)))
	}

	if logIt {
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
		} else {
			klog.InfoDepth(1, args...)
		}
	}
}

// Infof logs to the INFO log.
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
//
// See comments above for Info() for guidelines for inVerboseLevel.
func (l *logger) Infof(inVerboseLevel int32, inFormat string, args ...interface{}) {
	logIt := true
	if inVerboseLevel > 0 {
		logIt = bool(klog.V(klog.Level(inVerboseLevel)))
	}

	if logIt {
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
		} else {
			klog.InfoDepth(1, fmt.Sprintf(inFormat, args...))
		}
	}
}

func (l *logger) Infow(msg string, fields Fields) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.InfoDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fields))
	} else {
		klog.InfoDepth(1, fmt.Sprintf(msg+" %v", fields))
	}
}

// Warn logs to the WARNING and INFO logs.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Warnings are reserved for situations that indicate an inconsistency or an error that
// won't result in a departure of specifications, correctness, or expected behavior.
func (l *logger) Warn(args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
	} else {
		klog.WarningDepth(1, args...)
	}
}

// Warnf logs to the WARNING and INFO logs.
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
//
// See comments above for Warn() for guidelines on errors vs warnings.
func (l *logger) Warnf(inFormat string, args ...interface{}) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
	} else {
		klog.WarningDepth(1, fmt.Sprintf(inFormat, args...))
	}
}

func (l *logger) Warnw(msg string, fields Fields) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.WarningDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fields))
	} else {
		klog.WarningDepth(1, fmt.Sprintf(msg+" %v", fields))
	}
}

// Error logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled like fmt.Print(); a newline is appended if missing.
//
// Errors are reserved for situations that indicate an implementation deficiency, a
// corruption of data or resources, or an issue that if not addressed could spiral into deeper issues.
// Logging an error reflects that correctness or expected behavior is either broken or under threat.
func (l *logger) Error(args ...interface{}) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprint(args...))
		} else {
			klog.ErrorDepth(1, args...)
		}
	}
}

// Errorf logs to the ERROR, WARNING, and INFO logs.
// Arguments are handled like fmt.Print; a newline is appended if missing.
//
// See comments above for Error() for guidelines on errors vs warnings.
func (l *logger) Errorf(inFormat string, args ...interface{}) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
		} else {
			klog.ErrorDepth(1, fmt.Sprintf(inFormat, args...))
		}
	}
}

func (l *logger) Errorw(msg string, fields Fields) {
	if l.hasPrefix {
		padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
		klog.ErrorDepth(1, l.logPrefix, padding, fmt.Sprintf(msg+" %v", fields))
	} else {
		klog.ErrorDepth(1, fmt.Sprintf(msg+" %v", fields))
	}
}

// Fatalf logs to the FATAL, ERROR, WARNING, and INFO logs,
// Arguments are handled like fmt.Printf(); a newline is appended if missing.
func (l *logger) Fatalf(inFormat string, args ...interface{}) {
	{
		if l.hasPrefix {
			padding := strings.Repeat(" ", longestLabel-len(l.logPrefix))
			klog.FatalDepth(1, l.logPrefix, padding, fmt.Sprintf(inFormat, args...))
		} else {
			klog.FatalDepth(1, fmt.Sprintf(inFormat, args...))
		}
	}
}
