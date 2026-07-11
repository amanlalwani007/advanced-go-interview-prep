package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR"}[l]
}

type Logger struct {
	level  LogLevel
	fields map[string]interface{}
}

func NewLogger(level LogLevel) *Logger {
	return &Logger{level: level, fields: make(map[string]interface{})}
}

func (l *Logger) With(key string, value interface{}) *Logger {
	cp := &Logger{level: l.level, fields: make(map[string]interface{})}
	for k, v := range l.fields {
		cp.fields[k] = v
	}
	cp.fields[key] = value
	return cp
}

func (l *Logger) log(level LogLevel, msg string, args ...interface{}) {
	if level < l.level {
		return
	}
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level.String(),
		"message":   fmt.Sprintf(msg, args...),
	}
	for k, v := range l.fields {
		entry[k] = v
	}
	json.NewEncoder(os.Stdout).Encode(entry)
}

func (l *Logger) Debug(msg string, args ...interface{}) { l.log(DEBUG, msg, args...) }
func (l *Logger) Info(msg string, args ...interface{})  { l.log(INFO, msg, args...) }
func (l *Logger) Warn(msg string, args ...interface{})  { l.log(WARN, msg, args...) }
func (l *Logger) Error(msg string, args ...interface{}) { l.log(ERROR, msg, args...) }

func main() {
	log := NewLogger(INFO).With("service", "payment-api").With("env", "prod")

	log.Info("processing payment", "amount", 99.99, "currency", "USD")

	loggerWithRequest := log.With("request_id", "req-12345")
	loggerWithRequest.Debug("debug detail (hidden)")
	loggerWithRequest.Warn("retrying after timeout", "attempt", 2)

	log.Error("payment failed", "error", "insufficient funds")
}
