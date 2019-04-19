package webhook

import (
	"net/http"
)

var (
	errInvalidRequest      = newError("Invalid request", http.StatusUnprocessableEntity)
	errUnableToDeserialize = newError("Unable to deserialize request object", http.StatusBadRequest)
	errUnableToReadBody    = newError("Unable to read request body", http.StatusInternalServerError)
	errUnableToWriteConfig = newError("Unable to write telegraf.conf", http.StatusInternalServerError)
	errUnsupportedMedia    = newError("Expected a Content-Type of application/json", http.StatusUnsupportedMediaType)
)

type httpError struct {
	message string
	code    int
}

func newError(msg string, code int) *httpError {
	return &httpError{
		message: msg,
		code:    code,
	}
}

func (e *httpError) Error() string {
	return e.message
}

func (e *httpError) Write(w http.ResponseWriter) {
	http.Error(w, e.message, e.code)
}
