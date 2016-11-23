package logger

import (
	"net/http"
	"time"
)

// EventListener is a listener for a specific event as given by its flag.
type EventListener func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{})

// ErrorListener is a handler for error events.
type ErrorListener func(writer Logger, ts TimeSource, err error)

// NewErrorListener returns a new handler for EventFatalError and EventError events.
func NewErrorListener(listener ErrorListener) EventListener {
	return func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{}) {
		if len(state) > 0 {
			if typedError, isTyped := state[0].(error); isTyped {
				listener(writer, ts, typedError)
			}
		}
	}
}

// RequestStartListener is a listener for request events.
type RequestStartListener func(writer Logger, ts TimeSource, req *http.Request)

// NewRequestStartListener returns a new handler for request events.
func NewRequestStartListener(listener RequestStartListener) EventListener {
	return func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{}) {
		if len(state) > 0 {
			if typedRequest, isTyped := state[0].(*http.Request); isTyped {
				listener(writer, ts, typedRequest)
			}
		}
	}
}

// RequestListener is a listener for request events.
type RequestListener func(writer Logger, ts TimeSource, req *http.Request, statusCode, contentLengthBytes int, elapsed time.Duration)

// NewRequestListener returns a new handler for request events.
func NewRequestListener(listener RequestListener) EventListener {
	return func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{}) {
		if len(state) < 3 {
			return
		}

		req, err := stateAsRequest(state[0])
		if err != nil {
			return
		}

		statusCode, err := stateAsInteger(state[1])
		if err != nil {
			return
		}

		contentLengthBytes, err := stateAsInteger(state[2])
		if err != nil {
			return
		}

		elapsed, err := stateAsDuration(state[3])
		if err != nil {
			return
		}

		listener(writer, ts, req, statusCode, contentLengthBytes, elapsed)
	}
}

// RequestBodyListener is a listener for request bodies.
type RequestBodyListener func(writer Logger, ts TimeSource, body []byte)

// NewRequestBodyListener returns a new handler for request body events.
func NewRequestBodyListener(listener RequestBodyListener) EventListener {
	return func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{}) {
		if len(state) < 1 {
			return
		}
		body, err := stateAsBytes(state[0])
		if err != nil {
			return
		}
		listener(writer, ts, body)
	}
}

// ResponseListener is a handler for response body events.
type ResponseListener func(writer Logger, ts TimeSource, body []byte)

// NewResponseListener creates a new listener for response body events.
func NewResponseListener(listener ResponseListener) EventListener {
	return func(writer Logger, ts TimeSource, eventFlag EventFlag, state ...interface{}) {
		if len(state) < 1 {
			return
		}
		res, err := stateAsBytes(state[0])
		if err != nil {
			return
		}
		listener(writer, ts, res)
	}
}
