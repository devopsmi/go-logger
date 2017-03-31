package logger

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/blendlabs/go-workqueue"
)

var (
	// DefaultAgentQueueWorkers is the number of consumers (goroutines) for the agent work queue.
	DefaultAgentQueueWorkers = 4

	// DefaultAgentQueueLength is the maximum number of items to buffer in the event queue.
	DefaultAgentQueueLength = 1 << 20 // 1mm items
)

var (
	_default     *Agent
	_defaultLock sync.Mutex
)

var (
	// DefaultAgentVerbosity is the default verbosity for a diagnostics agent inited from the environment.
	DefaultAgentVerbosity = NewEventFlagSetWithEvents(EventFatalError, EventError, EventWebRequest, EventInfo)
)

// Default returnes a default Agent singleton.
func Default() *Agent {
	return _default
}

// SetDefault sets the diagnostics singleton.
func SetDefault(agent *Agent) {
	_defaultLock.Lock()
	defer _defaultLock.Unlock()
	_default = agent
}

// New returns a new diagnostics with a given bitflag verbosity.
func New(events *EventFlagSet, optionalWriter ...Logger) *Agent {
	diag := &Agent{
		events:         events,
		eventQueue:     newEventQueue(),
		eventListeners: map[EventFlag][]EventListener{},
	}

	if len(optionalWriter) > 0 {
		diag.writer = optionalWriter[0]
	} else {
		diag.writer = NewLogWriter(os.Stdout, os.Stderr)
	}
	return diag
}

// NewFromEnvironment returns a new diagnostics with a given bitflag verbosity.
func NewFromEnvironment() *Agent {
	return New(NewEventFlagSetFromEnvironment(), NewLogWriterFromEnvironment())
}

// Agent is a handler for various logging events with descendent handlers.
type Agent struct {
	writer             Logger
	events             *EventFlagSet
	eventListenersLock sync.Mutex
	eventListeners     map[EventFlag][]EventListener
	eventQueue         *workqueue.Queue
}

// Writer returns the inner Logger for the diagnostics agent.
func (da *Agent) Writer() Logger {
	return da.writer
}

// EventQueue returns the inner event queue for the agent.
func (da *Agent) EventQueue() *workqueue.Queue {
	return da.eventQueue
}

// Events returns the EventFlagSet
func (da *Agent) Events() *EventFlagSet {
	return da.events
}

// SetVerbosity sets the agent verbosity synchronously.
func (da *Agent) SetVerbosity(events *EventFlagSet) {
	da.events = events
}

// EnableEvent flips the bit flag for a given event.
func (da *Agent) EnableEvent(eventFlag EventFlag) {
	da.events.Enable(eventFlag)
}

// DisableEvent flips the bit flag for a given event.
func (da *Agent) DisableEvent(eventFlag EventFlag) {
	da.events.Disable(eventFlag)
}

// IsEnabled asserts if a flag value is set or not.
func (da *Agent) IsEnabled(flagValue EventFlag) bool {
	if da == nil {
		return false
	}
	return da.events.IsEnabled(flagValue)
}

// HasListener returns if there are registered listener for an event.
func (da *Agent) HasListener(event EventFlag) bool {
	if da == nil {
		return false
	}
	if da.eventListeners == nil {
		return false
	}
	listeners, hasHandler := da.eventListeners[event]
	if !hasHandler {
		return false
	}
	return len(listeners) > 0
}

// AddEventListener adds a listener for errors.
func (da *Agent) AddEventListener(eventFlag EventFlag, listener EventListener) {
	da.eventListenersLock.Lock()
	da.eventListeners[eventFlag] = append(da.eventListeners[eventFlag], listener)
	da.eventListenersLock.Unlock()
}

// RemoveListeners clears *all* listeners for an EventFlag.
func (da *Agent) RemoveListeners(eventFlag EventFlag) {
	delete(da.eventListeners, eventFlag)
}

// OnEvent fires the currently configured event listeners.
func (da *Agent) OnEvent(eventFlag EventFlag, state ...interface{}) {
	if da == nil {
		return
	}
	if da.IsEnabled(eventFlag) && da.HasListener(eventFlag) {
		da.eventQueue.Enqueue(da.triggerListeners, append([]interface{}{TimeNow(), eventFlag}, state...)...)
	}
}

// Infof logs an informational message to the output stream.
func (da *Agent) Infof(format string, args ...interface{}) {
	if da.IsEnabled(EventInfo) {
		da.queueWrite(EventInfo, ColorWhite, format, args...)

		if da.HasListener(EventInfo) {
			da.eventQueue.Enqueue(da.triggerListeners, append([]interface{}{TimeNow(), EventInfo, format}, args...)...)
		}
	}
}

// Debugf logs a debug message to the output stream.
func (da *Agent) Debugf(format string, args ...interface{}) {
	if da.IsEnabled(EventDebug) {
		da.queueWrite(EventDebug, ColorLightYellow, format, args...)

		if da.HasListener(EventDebug) {
			da.eventQueue.Enqueue(da.triggerListeners, append([]interface{}{TimeNow(), EventDebug, format}, args...)...)
		}
	}
}

// Warningf logs a debug message to the output stream.
func (da *Agent) Warningf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	if da.IsEnabled(EventWarning) {
		da.queueWriteError(EventWarning, ColorRed, format, args...)
		if da.HasListener(EventWarning) {
			da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventWarning, err})
		}
	}
	return err
}

// Warning logs a warning error to std err.
func (da *Agent) Warning(err error) error {
	if err != nil {
		if da.IsEnabled(EventWarning) {
			da.queueWriteError(EventWarning, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventWarning) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventWarning, err})
			}
		}
	}
	return err
}

// WarningWithReq logs a warning error to std err with a request.
func (da *Agent) WarningWithReq(err error, req *http.Request) error {
	if err != nil {
		if da.IsEnabled(EventWarning) {
			da.queueWriteError(EventWarning, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventWarning) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventWarning, err, req})
			}
		}
	}
	return err
}

// Errorf writes an event to the log and triggers event listeners.
func (da *Agent) Errorf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	if da.IsEnabled(EventError) {
		da.queueWriteError(EventError, ColorRed, format, args...)
		if da.HasListener(EventError) {
			da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventError, err})
		}
	}
	return err
}

// Fatal logs an error to std err.
func (da *Agent) Error(err error) error {
	if err != nil {
		if da.IsEnabled(EventError) {
			da.queueWriteError(EventError, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventError) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventError, err})
			}
		}
	}
	return err
}

// ErrorWithReq logs an error to std err with a request.
func (da *Agent) ErrorWithReq(err error, req *http.Request) error {
	if err != nil {
		if da.IsEnabled(EventError) {
			da.queueWriteError(EventError, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventError) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventError, err, req})
			}
		}
	}
	return err
}

// Fatalf writes an event to the log and triggers event listeners.
func (da *Agent) Fatalf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	if da.IsEnabled(EventFatalError) {
		da.queueWriteError(EventFatalError, ColorRed, format, args...)
		if da.HasListener(EventFatalError) {
			da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventFatalError, err})
		}
	}
	return err
}

// Fatal logs the result of a panic to std err.
func (da *Agent) Fatal(err error) error {
	if err != nil {
		if da.IsEnabled(EventFatalError) {
			da.queueWriteError(EventFatalError, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventFatalError) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventFatalError, err})
			}
		}
	}
	return err
}

// FatalWithReq logs the result of a fatal error to std err with a request.
func (da *Agent) FatalWithReq(err error, req *http.Request) error {
	if err != nil {
		if da.IsEnabled(EventFatalError) {
			da.queueWriteError(EventFatalError, ColorRed, fmt.Sprintf("%+v", err))
			if da.HasListener(EventFatalError) {
				da.eventQueue.Enqueue(da.triggerListeners, []interface{}{TimeNow(), EventFatalError, err, req})
			}
		}
	}
	return err
}

// Close releases shared resources for the agent.
func (da *Agent) Close() error {
	return da.eventQueue.Close()
}

// Drain waits for the agent to finish it's queue of events before closing.
func (da *Agent) Drain() error {
	da.SetVerbosity(NewEventFlagSetNone())

	for da.eventQueue.Len() > 0 {
		time.Sleep(time.Millisecond)
	}
	return da.Close()
}

// --------------------------------------------------------------------------------
// internal methods
// --------------------------------------------------------------------------------

// triggerListeners triggers the currently configured event listeners.
func (da *Agent) triggerListeners(actionState ...interface{}) error {
	if len(actionState) < 2 {
		return nil
	}

	timeSource, err := stateAsTimeSource(actionState[0])
	if err != nil {
		return err
	}

	eventFlag, err := stateAsEventFlag(actionState[1])
	if err != nil {
		return err
	}

	da.eventListenersLock.Lock()
	listeners := da.eventListeners[eventFlag]
	da.eventListenersLock.Unlock()

	for x := 0; x < len(listeners); x++ {
		listener := listeners[x]
		listener(da.writer, timeSource, eventFlag, actionState[2:]...)
	}

	return nil
}

// printf checks an event flag and writes a message with a given color.
func (da *Agent) queueWrite(eventFlag EventFlag, color AnsiColorCode, format string, args ...interface{}) {
	if len(format) > 0 {
		da.eventQueue.Enqueue(da.write, append([]interface{}{TimeNow(), eventFlag, color, format}, args...)...)
	}
}

// errorf checks an event flag and writes a message to the error stream (if one is configured) with a given color.
func (da *Agent) queueWriteError(eventFlag EventFlag, color AnsiColorCode, format string, args ...interface{}) {
	if len(format) > 0 {
		da.eventQueue.Enqueue(da.writeError, append([]interface{}{TimeNow(), eventFlag, color, format}, args...)...)
	}
}

func (da *Agent) write(actionState ...interface{}) error {
	return da.writeWithOutput(da.writer.PrintfWithTimeSource, actionState...)
}

func (da *Agent) writeError(actionState ...interface{}) error {
	return da.writeWithOutput(da.writer.ErrorfWithTimeSource, actionState...)
}

// writeEventMessage writes an event message.
func (da *Agent) writeWithOutput(output loggerOutputWithTimeSource, actionState ...interface{}) error {
	if len(actionState) < 4 {
		return nil
	}

	timeSource, err := stateAsTimeSource(actionState[0])
	if err != nil {
		return err
	}

	eventFlag, err := stateAsEventFlag(actionState[1])
	if err != nil {
		return err
	}

	labelColor, err := stateAsAnsiColorCode(actionState[2])
	if err != nil {
		return err
	}

	format, err := stateAsString(actionState[3])
	if err != nil {
		return err
	}

	_, err = output(timeSource, "%s %s", da.writer.Colorize(string(eventFlag), labelColor), fmt.Sprintf(format, actionState[4:]...))
	return err
}

func newEventQueue() *workqueue.Queue {
	eq := workqueue.NewWithWorkers(DefaultAgentQueueWorkers)
	eq.SetMaxWorkItems(DefaultAgentQueueLength) //more than this and queuing will block
	eq.Start()
	return eq
}
