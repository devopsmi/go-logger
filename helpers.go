package logger

import (
	"net/http"
	"strconv"
	"time"
)

// WriteRequest is a helper method to write request start events to a writer.
func WriteRequest(writer Logger, req *http.Request) {
	buffer := writer.GetBuffer()
	defer writer.PutBuffer(buffer)

	buffer.WriteString(writer.Colorize("Request", ColorGreen))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(GetIP(req))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(writer.Colorize(req.Method, ColorBlue))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(req.URL.Path)
	buffer.WriteRune(RuneSpace)

	writer.Write(buffer.Bytes())
}

// WriteRequestComplete is a helper method to write request complete events to a writer.
func WriteRequestComplete(writer Logger, req *http.Request, statusCode, contentLengthBytes int, elapsed time.Duration) {
	buffer := writer.GetBuffer()
	defer writer.PutBuffer(buffer)

	buffer.WriteString(writer.Colorize("Request Complete", ColorGreen))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(GetIP(req))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(writer.Colorize(req.Method, ColorBlue))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(req.URL.Path)
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(writer.ColorizeByStatusCode(statusCode, strconv.Itoa(statusCode)))
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(elapsed.String())
	buffer.WriteRune(RuneSpace)
	buffer.WriteString(FormatFileSize(contentLengthBytes))

	writer.Write(buffer.Bytes())
}

// WriteRequestBody is a helper method to write request start events to a writer.
func WriteRequestBody(writer Logger, body []byte) {
	buffer := writer.GetBuffer()
	defer writer.PutBuffer(buffer)
	buffer.WriteString(writer.Colorize("Request Body", ColorGreen))
	buffer.WriteRune(RuneSpace)
	buffer.Write(body)

	writer.Write(buffer.Bytes())
}