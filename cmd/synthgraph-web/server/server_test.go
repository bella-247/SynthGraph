package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer() *Server {
	return New("", "") // in-memory, no persistence
}

func TestHealthEndpoint(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var responseBody map[string]string
	if decodeError := json.NewDecoder(recorder.Body).Decode(&responseBody); decodeError != nil {
		t.Fatalf("failed to decode response: %v", decodeError)
	}
	if responseBody["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", responseBody["status"])
	}
}

func TestFrontendServesIndexHTML(t *testing.T) {
	serverWithHTML := New("<html>test</html>", "")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	serverWithHTML.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %q", contentType)
	}

	if recorder.Body.String() != "<html>test</html>" {
		t.Errorf("expected embedded HTML, got %q", recorder.Body.String())
	}
}

func TestFrontendReturns404ForUnknownPaths(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestParseEndpointRequiresSQL(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	requestBody := parseRequest{SQL: ""}
	bodyBytes, _ := json.Marshal(requestBody)
	request := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader(bodyBytes))
	request.Header.Set("Content-Type", "application/json")
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for empty SQL, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestParseEndpointReturnsErrorForInvalidJSON(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader([]byte("not json")))
	request.Header.Set("Content-Type", "application/json")
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for invalid JSON, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestJobsEndpoint(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var jobs []jobSummary
	if decodeError := json.NewDecoder(recorder.Body).Decode(&jobs); decodeError != nil {
		t.Fatalf("failed to decode response: %v", decodeError)
	}
	if len(jobs) != 0 {
		t.Errorf("expected empty job list, got %d items", len(jobs))
	}
}

func TestJobStoreAddAndList(t *testing.T) {
	jobStore := NewJobStore()
	firstJob := &Job{
		Status: "completed",
		Tables: 3,
		Format: "csv",
		Data:   []byte("a,b,c"),
	}
	secondJob := &Job{
		Status: "completed",
		Tables: 5,
		Format: "sql",
		Data:   []byte("INSERT INTO"),
	}
	jobStore.Add(firstJob)
	jobStore.Add(secondJob)

	jobs := jobStore.List()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].ID != 2 {
		t.Errorf("expected job 2 first (newest first), got job %d", jobs[0].ID)
	}
	if jobs[0].Tables != 5 {
		t.Errorf("expected 5 tables for first job, got %d", jobs[0].Tables)
	}
	if jobs[1].ID != 1 {
		t.Errorf("expected job 1 second, got job %d", jobs[1].ID)
	}
}

func TestCORSHeaders(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	request.Header.Set("Origin", "http://example.com")
	testServer.ServeHTTP(recorder, request)

	allowedOrigin := recorder.Header().Get("Access-Control-Allow-Origin")
	if allowedOrigin != "*" {
		t.Errorf("expected CORS origin '*', got %q", allowedOrigin)
	}
}

func TestCORSPreflight(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("Access-Control-Request-Method", "POST")
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Errorf("expected status %d for OPTIONS, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestPanicRecovery(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewReader([]byte("{}")))
	request.Header.Set("Content-Type", "application/json")
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestRequestBodySizeLimit(t *testing.T) {
	testServer := newTestServer()
	hugePayload := strings.Repeat("x", 12<<20)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader([]byte(hugePayload)))
	request.Header.Set("Content-Type", "application/json")
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for oversized body, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestWriteJSONHelper(t *testing.T) {
	recorder := httptest.NewRecorder()
	testData := map[string]string{"key": "value"}
	writeJSON(recorder, http.StatusCreated, testData)

	if recorder.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, recorder.Code)
	}

	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("expected application/json, got %q", contentType)
	}

	var decoded map[string]string
	if decodeError := json.NewDecoder(recorder.Body).Decode(&decoded); decodeError != nil {
		t.Fatalf("failed to decode: %v", decodeError)
	}
	if decoded["key"] != "value" {
		t.Errorf("expected 'value', got %q", decoded["key"])
	}
}

func TestWriteErrorHelper(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeError(recorder, http.StatusNotFound, "resource %s not found", "job")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}

	var decoded map[string]string
	if decodeError := json.NewDecoder(recorder.Body).Decode(&decoded); decodeError != nil {
		t.Fatalf("failed to decode: %v", decodeError)
	}
	if decoded["error"] != "resource job not found" {
		t.Errorf("expected 'resource job not found', got %q", decoded["error"])
	}
}

func TestJobStoreConcurrencySafety(t *testing.T) {
	jobStore := NewJobStore()
	doneChannel := make(chan bool, 100)

	for index := 0; index < 100; index++ {
		go func() {
			jobStore.Add(&Job{Status: "completed"})
			doneChannel <- true
		}()
	}

	for index := 0; index < 100; index++ {
		<-doneChannel
	}

	jobs := jobStore.List()
	if len(jobs) != 100 {
		t.Errorf("expected 100 jobs, got %d", len(jobs))
	}

	seenIDs := make(map[int]bool)
	for _, job := range jobs {
		if seenIDs[job.ID] {
			t.Errorf("duplicate job ID: %d", job.ID)
		}
		seenIDs[job.ID] = true
	}
}
