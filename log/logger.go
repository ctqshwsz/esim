package log

import (
	"context"

	"go.uber.org/zap"
	glogger "gorm.io/gorm/logger"
)

type Elogger struct {
	debug bool

	json bool

	ez *EsimZap

	Elogger *zap.Logger

	sugar *zap.SugaredLogger

	glog *gormLogger
}

type LoggerOptions struct{}

type Option func(c *Elogger)

func NewLogger(options ...Option) Logger {
	Elogger := &Elogger{}

	for _, option := range options {
		option(Elogger)
	}

	if Elogger.ez == nil {
		Elogger.ez = NewEsimZap(
			WithEsimZapDebug(Elogger.debug),
			WithEsimZapJSON(Elogger.json),
		)
	}

	glog := NewGormLogger(
		WithGLogEsimZap(Elogger.ez),
	)

	Elogger.glog = glog.(*gormLogger)

	Elogger.Elogger = Elogger.ez.Logger
	Elogger.sugar = Elogger.ez.Logger.Sugar()

	return Elogger
}

func WithDebug(debug bool) Option {
	return func(l *Elogger) {
		l.debug = debug
	}
}

func WithJSON(json bool) Option {
	return func(l *Elogger) {
		l.json = json
	}
}

func WithEsimZap(ez *EsimZap) Option {
	return func(l *Elogger) {
		l.ez = ez
	}
}

func (log *Elogger) Error(msg string) {
	log.Elogger.Error(msg)
}

func (log *Elogger) Glog() glogger.Interface {
	return log.glog
}

func (log *Elogger) Debugf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Debugf(template, args...)
}

func (log *Elogger) Infof(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Infof(template, args...)
}

func (log *Elogger) Warnf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Warnf(template, args...)
}

func (log *Elogger) Errorf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Errorf(template, args...)
}

func (log *Elogger) DPanicf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).DPanicf(template, args...)
}

func (log *Elogger) Panicf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Panicf(template, args...)
}

func (log *Elogger) Fatalf(template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(context.TODO())...).Fatalf(template, args...)
}

func (log *Elogger) Debugc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Debugf(template, args...)
}

func (log *Elogger) Infoc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Infof(template, args...)
}

func (log *Elogger) Warnc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Warnf(template, args...)
}

func (log *Elogger) Errorc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Errorf(template, args...)
}

func (log *Elogger) DPanicc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).DPanicf(template, args...)
}

func (log *Elogger) Panicc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Panicf(template, args...)
}

func (log *Elogger) Fatalc(ctx context.Context, template string, args ...interface{}) {
	log.sugar.With(log.ez.getArgs(ctx)...).Fatalf(template, args...)
}
