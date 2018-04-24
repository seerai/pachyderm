package log

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/camelcase"
	"github.com/sirupsen/logrus"
)

// Logger is a helper for emitting our grpc API logs
type Logger interface {
	Log(request interface{}, response interface{}, err error, duration time.Duration)
	LogAtLevelFromDepth(request interface{}, response interface{}, err error, duration time.Duration, level logrus.Level, depth int)
}

type logger struct {
	*logrus.Entry
}

// NewLogger creates a new logger
func NewLogger(service string) Logger {
	l := logrus.New()
	l.Formatter = new(prettyFormatter)
	return &logger{
		l.WithFields(logrus.Fields{"service": service}),
	}
}

// Helper function used to log requests and responses from our GRPC method
// implementations
func (l *logger) Log(request interface{}, response interface{}, err error, duration time.Duration) {
	state := "started"
	if err != nil {
		l.LogAtLevelFromDepth(request, response, err, duration, logrus.ErrorLevel, 4)
		state = "errored"
	} else {
		l.LogAtLevelFromDepth(request, response, err, duration, logrus.InfoLevel, 4)
	}
	if duration.Seconds() > 0 {
		state = "finished"
	}
	l.ReportMetric(state, duration)
}

func (l *logger) ReportMetric(state string, duration time.Duration) {
	depth := 4
	pc := make([]uintptr, depth)
	runtime.Callers(depth, pc)
	split := strings.Split(runtime.FuncForPC(pc[0]).Name(), ".")
	method := split[len(split)-1]
	fmt.Printf("GOING TO REPORT STATS FOR METHOD: %v\n", method)
	fmt.Printf("state: %v, duration: %v\n", state, duration)

	var tokens []string
	for _, token := range camelcase.Split(method) {
		tokens = append(tokens, strings.ToLower(token))
	}
	rootStatName := string.Join(tokens, "_")

	bucketFactor = 2.0
	bucketCount = 20 // Which makes the max bucket 2^20 seconds or ~12 days in size
	runTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "pachyderm",
			Subsystem: "pachd",
			Name:      fmt.Sprintf("%v_time", rootStatName),
			Help:      fmt.Sprintf("Run time of %v", method),
			Buckets:   prometheus.ExponentialBuckets(1.0, bucketFactor, bucketCount),
		},
		[]string{
			"state", // Since both finished and errored datums can have proc times
		},
	)
	if err := prometheus.Register(runTime); err != nil {
		fmt.Printf("error registering prometheus metric: %v\n", err)
	}
	if hist, err := runTime.GetMetricWithLabelValues(state); err != nil {
		logger.Logf("failed to get histogram w labels: state (%v) with error %v", state, err)
	} else {
		hist.Observe(duration.Seconds())
	}

	secondsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "pachyderm",
			Subsystem: "pachd",
			Name:      fmt.Sprintf("%v_seconds_count", rootStatName),
			Help:      fmt.Sprintf("cumulative number of seconds spent in %v", method),
		},
	)
	if err := prometheus.Register(secondsCount); err != nil {
		fmt.Printf("error registering prometheus metric: %v\n", err)
	}
	secondsCount.Add(duration.Seconds())

}

func (l *logger) LogAtLevelFromDepth(request interface{}, response interface{}, err error, duration time.Duration, level logrus.Level, depth int) {
	pc := make([]uintptr, depth)
	runtime.Callers(depth, pc)
	split := strings.Split(runtime.FuncForPC(pc[0]).Name(), ".")
	method := split[len(split)-1]

	fields := logrus.Fields{
		"method":  method,
		"request": request,
	}
	if response != nil {
		fields["response"] = response
	}
	if err != nil {
		// "err" itself might be a code or even an empty struct
		fields["error"] = err.Error()
	}
	if duration > 0 {
		fields["duration"] = duration
	}
	entry := l.WithFields(fields)

	switch level {
	case logrus.PanicLevel:
		entry.Panic()
	case logrus.FatalLevel:
		entry.Fatal()
	case logrus.ErrorLevel:
		entry.Error()
	case logrus.WarnLevel:
		entry.Warn()
	case logrus.InfoLevel:
		entry.Info()
	case logrus.DebugLevel:
		entry.Debug()
	}
}

type prettyFormatter struct{}

func (f *prettyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	serialized := []byte(
		fmt.Sprintf(
			"%v %v ",
			entry.Time.Format(logrus.DefaultTimestampFormat),
			strings.ToUpper(entry.Level.String()),
		),
	)
	if entry.Data["service"] != nil {
		serialized = append(serialized, []byte(fmt.Sprintf("%v.%v ", entry.Data["service"], entry.Data["method"]))...)
	}
	if len(entry.Data) > 2 {
		delete(entry.Data, "service")
		delete(entry.Data, "method")
		if entry.Data["duration"] != nil {
			entry.Data["duration"] = entry.Data["duration"].(time.Duration).Seconds()
		}
		data, err := json.Marshal(entry.Data)
		if err != nil {
			return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
		}
		serialized = append(serialized, []byte(string(data))...)
		serialized = append(serialized, ' ')
	}

	serialized = append(serialized, []byte(entry.Message)...)
	serialized = append(serialized, '\n')
	return serialized, nil
}
