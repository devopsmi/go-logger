package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	exception "github.com/blendlabs/go-exception"
	logger "github.com/blendlabs/go-logger"
)

var pool = logger.NewBufferPool(16)

func logged(handler http.HandlerFunc) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		start := time.Now()
		logger.Diagnostics().OnEvent(logger.EventRequest, req)
		rw := logger.NewResponseWriter(res)
		handler(rw, req)
		logger.Diagnostics().OnEvent(logger.EventRequestComplete, req, rw.StatusCode(), rw.ContentLength(), time.Now().Sub(start))
	}
}

func indexHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(`{"status":"ok!"}`))
}

func fatalErrorHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusInternalServerError)
	logger.Diagnostics().Fatal(exception.New("this is an exception"))
	res.Write([]byte(`{"status":"not ok."}`))
}

func errorHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusInternalServerError)
	logger.Diagnostics().Error(exception.New("this is an exception"))
	res.Write([]byte(`{"status":"not ok."}`))
}

func warningHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusInternalServerError)
	logger.Diagnostics().Warning(exception.New("this is an exception"))
	res.Write([]byte(`{"status":"not ok."}`))
}

func postHandler(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	b := pool.Get()
	defer req.Body.Close()
	defer pool.Put(b)
	b.ReadFrom(req.Body)
	res.Write([]byte(fmt.Sprintf(`{"status":"ok!","received_bytes":%d}`, b.Len())))
	logger.Diagnostics().OnEvent(logger.EventRequestBody, b.Bytes())
}

func port() string {
	envPort := os.Getenv("PORT")
	if len(envPort) > 0 {
		return envPort
	}
	return "8888"
}

func main() {
	logger.InitializeDiagnostics(logger.EventAll, logger.NewLogWriter(os.Stdout, os.Stderr))
	logger.Diagnostics().AddEventListener(logger.EventRequest, logger.NewRequestHandler(func(writer logger.Logger, req *http.Request) {
		logger.WriteRequest(writer, req)
	}))
	logger.Diagnostics().AddEventListener(logger.EventRequestComplete, logger.NewRequestCompleteHandler(func(writer logger.Logger, req *http.Request, statusCode, contentLengthBytes int, elapsed time.Duration) {
		logger.WriteRequestComplete(writer, req, statusCode, contentLengthBytes, elapsed)
	}))
	logger.Diagnostics().AddEventListener(logger.EventRequestBody, logger.NewRequestBodyHandler(func(writer logger.Logger, body []byte) {
		logger.WriteRequestBody(writer, body)
	}))

	http.HandleFunc("/", logged(indexHandler))
	http.HandleFunc("/fatalerror", logged(fatalErrorHandler))
	http.HandleFunc("/error", logged(errorHandler))
	http.HandleFunc("/warning", logged(warningHandler))
	http.HandleFunc("/post", logged(postHandler))
	logger.Diagnostics().Infof("Listening on :%s", port())
	log.Fatal(http.ListenAndServe(":"+port(), nil))
}