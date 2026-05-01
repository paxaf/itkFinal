package logger

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type LoggerSuite struct {
	suite.Suite
}

func TestLoggerSuite(t *testing.T) {
	suite.Run(t, new(LoggerSuite))
}

func (s *LoggerSuite) SetupTest() {
	globalLoggerMu.Lock()
	globalLogger = nil
	globalLoggerMu.Unlock()
	s.T().Setenv("LOG_LEVEL", "")
}

func (s *LoggerSuite) TestGetLogLevel() {
	tests := map[string]zerolog.Level{
		"debug":   zerolog.DebugLevel,
		"info":    zerolog.InfoLevel,
		"warn":    zerolog.WarnLevel,
		"error":   zerolog.ErrorLevel,
		"fatal":   zerolog.FatalLevel,
		"unknown": zerolog.InfoLevel,
	}

	for level, expected := range tests {
		s.Run(level, func() {
			s.Require().Equal(expected, getLogLevel(level))
		})
	}
}

func (s *LoggerSuite) TestGetDefaultLevel() {
	s.Require().Equal("info", getDefaultLevel())

	s.T().Setenv("LOG_LEVEL", "debug")
	s.Require().Equal("debug", getDefaultLevel())
}

func (s *LoggerSuite) TestNewStoresGlobalLogger() {
	l := New("error")

	globalLoggerMu.RLock()
	stored := globalLogger
	globalLoggerMu.RUnlock()

	s.Require().Same(l, stored)
}

func (s *LoggerSuite) TestGetOrInitGlobalLoggerCreatesAndReusesLogger() {
	s.T().Setenv("LOG_LEVEL", "debug")

	first := getOrInitGlobalLogger()
	second := getOrInitGlobalLogger()

	s.Require().NotNil(first)
	s.Require().Same(first, second)
}

func (s *LoggerSuite) TestLoggerMethods() {
	l := New("error")

	l.Debug(errors.New("debug error"))
	l.Debug("debug fields", map[string]interface{}{"key": "value"})
	l.Debug(123)
	l.Info("hello %s", "world")
	l.Info("info fields", map[string]interface{}{"key": "value"})
	l.Warn("plain warning")
	l.Warn("warn fields", map[string]interface{}{"key": "value"})
	l.Error("plain error")
	l.Error("error args", 1, "two")
}

func (s *LoggerSuite) TestGlobalLoggerMethods() {
	New("error")

	Debug("debug")
	Info("info")
	Warn("warn")
	Error("error")
}
