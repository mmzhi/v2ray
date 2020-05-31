package log

import (
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"io"
	"log"
	"os"
	"time"

	"v2ray.com/core/common/platform"
	"v2ray.com/core/common/signal/done"
	"v2ray.com/core/common/signal/semaphore"
)

// Writer is the interface for writing logs.
type Writer interface {
	Write(string) error
	io.Closer
}

// WriterCreator is a function to create LogWriters.
type WriterCreator func() Writer

type generalLogger struct {
	creator WriterCreator
	buffer  chan Message
	access  *semaphore.Instance
	done    *done.Instance
}

// NewLogger returns a generic log handler that can handle all type of messages.
func NewLogger(logWriterCreator WriterCreator) Handler {
	return &generalLogger{
		creator: logWriterCreator,
		buffer:  make(chan Message, 16),
		access:  semaphore.New(1),
		done:    done.New(),
	}
}

func (l *generalLogger) run() {
	defer l.access.Signal()

	dataWritten := false
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	logger := l.creator()
	if logger == nil {
		return
	}
	defer logger.Close() // nolint: errcheck

	for {
		select {
		case <-l.done.Wait():
			return
		case msg := <-l.buffer:
			logger.Write(msg.String() + platform.LineSeparator()) // nolint: errcheck
			dataWritten = true
		case <-ticker.C:
			if !dataWritten {
				return
			}
			dataWritten = false
		}
	}
}

func (l *generalLogger) Handle(msg Message) {
	select {
	case l.buffer <- msg:
	default:
	}

	select {
	case <-l.access.Wait():
		go l.run()
	default:
	}
}

func (l *generalLogger) Close() error {
	return l.done.Close()
}

type consoleLogWriter struct {
	logger *log.Logger
}

func (w *consoleLogWriter) Write(s string) error {
	w.logger.Print(s)
	return nil
}

func (w *consoleLogWriter) Close() error {
	return nil
}

type fileLogWriter struct {
	writer   *rotatelogs.RotateLogs
	logger *log.Logger
}

func (w *fileLogWriter) Write(s string) error {
	w.logger.Print(s)
	return nil
}

func (w *fileLogWriter) Close() error {
	return w.writer.Close()
}

// CreateStdoutLogWriter returns a LogWriterCreator that creates LogWriter for stdout.
func CreateStdoutLogWriter() WriterCreator {
	return func() Writer {
		return &consoleLogWriter{
			logger: log.New(os.Stdout, "", log.Ldate|log.Ltime),
		}
	}
}

// CreateStderrLogWriter returns a LogWriterCreator that creates LogWriter for stderr.
func CreateStderrLogWriter() WriterCreator {
	return func() Writer {
		return &consoleLogWriter{
			logger: log.New(os.Stderr, "", log.Ldate|log.Ltime),
		}
	}
}

// CreateFileLogWriter returns a LogWriterCreator that creates LogWriter for the given file.
func CreateFileLogWriter(path string) (WriterCreator, error) {
	writer, err := rotatelogs.New(
		path + ".%Y%m%d",
		rotatelogs.WithMaxAge(time.Hour * 24 * 30),             	// 文件最大保存时间
		rotatelogs.WithRotationTime(time.Hour * 24), 				// 日志切割时间间隔
		rotatelogs.WithClock(rotatelogs.Local),
	)
	if err != nil {
		return nil, err
	}
	writer.Close()
	return func() Writer {

		writer, err := rotatelogs.New(
			path + ".%Y%m%d",
			rotatelogs.WithMaxAge(time.Hour * 24 * 30),             	// 文件最大保存时间
			rotatelogs.WithRotationTime(time.Hour * 24), 				// 日志切割时间间隔
			rotatelogs.WithClock(rotatelogs.Local),
		)

		if err != nil {
			return nil
		}

		return &fileLogWriter{
			writer:   writer,
			logger: log.New(writer, "", log.Ldate|log.Ltime),
		}
	}, nil
}

func init() {
	RegisterHandler(NewLogger(CreateStdoutLogWriter()))
}
