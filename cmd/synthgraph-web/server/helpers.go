package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeJSON(responseWriter http.ResponseWriter, statusCode int, value interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	json.NewEncoder(responseWriter).Encode(value)
}

func writeError(responseWriter http.ResponseWriter, statusCode int, format string, arguments ...interface{}) {
	writeJSON(responseWriter, statusCode, errorResponse{
		Error: apiError{
			Code:    httpCodeString(statusCode),
			Message: fmt.Sprintf(format, arguments...),
		},
	})
}

func httpCodeString(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	case http.StatusRequestEntityTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusRequestTimeout:
		return "TIMEOUT"
	case http.StatusUnprocessableEntity:
		return "UNPROCESSABLE"
	default:
		return fmt.Sprintf("HTTP_%d", code)
	}
}

func decodeJSONBody(request *http.Request, target interface{}) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if decodeError := decoder.Decode(target); decodeError != nil {
		return fmt.Errorf("invalid JSON body: %w", decodeError)
	}
	return nil
}
