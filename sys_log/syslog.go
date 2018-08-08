package syslog

import (
	"os"
	"log"
	"io"
	"log/syslog"
)

type Logger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
	fatal *log.Logger
	aws   *log.Logger
}

func New(address, tag string) (*Logger, error) {
	var multiWriter io.Writer = os.Stdout
	if address != "" {
		sysLogWriter, err := syslog.Dial("udp", address, syslog.LOG_EMERG|syslog.LOG_KERN, tag)
		if err != nil {
			return nil, err
		}
		multiWriter = io.MultiWriter(sysLogWriter, os.Stdout)
	}
	l := Logger{}
	l.debug = log.New(os.Stdout, "DEBUG ", log.Ldate|log.Lmicroseconds|log.LUTC)
	l.info = log.New(multiWriter, "INFO ", log.Ldate|log.Lmicroseconds|log.LUTC)
	l.warn = log.New(multiWriter, "WARNING ", log.Ldate|log.Lmicroseconds|log.LUTC)
	l.error = log.New(multiWriter, "ERROR ", log.Ldate|log.Lmicroseconds|log.LUTC)
	l.fatal = log.New(multiWriter, "FATAL ", log.Ldate|log.Lmicroseconds|log.LUTC)
	l.aws = log.New(multiWriter, "AWS SDK ", log.Ldate|log.Lmicroseconds|log.LUTC)
	return &l, nil
}

func (l *Logger) Debugf(s string, args ...interface{}) {
	l.debug.Printf(s, args...)
}

func (l *Logger) Debugln(s string) {
	l.debug.Println(s)
}

func (l *Logger) Infof(s string, args ...interface{}) {
	l.info.Printf(s, args...)
}

func (l *Logger) Infoln(s string) {
	l.info.Println(s)
}

func (l *Logger) Warnf(s string, args ...interface{}) {
	l.warn.Printf(s, args...)
}

func (l *Logger) Warnln(s string) {
	l.warn.Println(s)
}

func (l *Logger) Errorf(s string, args ...interface{}) {
	l.error.Printf(s, args...)
}

func (l *Logger) Errorln(s string) {
	l.error.Printf(s)
}

func (l *Logger) Fatalf(s string, args ...interface{}) {
	l.fatal.Printf(s, args...)
	os.Exit(1)
}

func (l *Logger) Fatalln(s string) {
	l.fatal.Println(s)
	os.Exit(1)
}

func (l *Logger) AwsLog(args ...interface{}) {
	l.aws.Println(args...)
}

