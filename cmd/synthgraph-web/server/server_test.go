package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {}

func newTestServer() *Server {
	return New("", "")
}

func sseRequest(path string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Accept", "text/event-stream")
	return request
}

func TestHealthEndpoint(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var responseBody map[string]interface{}
	if decodeError := json.NewDecoder(recorder.Body).Decode(&responseBody); decodeError != nil {
		t.Fatalf("failed to decode response: %v", decodeError)
	}
	if responseBody["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", responseBody["status"])
	}
	if _, hasVersion := responseBody["version"]; !hasVersion {
		t.Error("expected version field in health response")
	}
	if _, hasUptime := responseBody["uptime"]; !hasUptime {
		t.Error("expected uptime field in health response")
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

	var decoded struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if decodeError := json.NewDecoder(recorder.Body).Decode(&decoded); decodeError != nil {
		t.Fatalf("failed to decode: %v", decodeError)
	}
	if decoded.Error.Message != "resource job not found" {
		t.Errorf("expected 'resource job not found', got %q", decoded.Error.Message)
	}
	if decoded.Error.Code != "NOT_FOUND" {
		t.Errorf("expected code 'NOT_FOUND', got %q", decoded.Error.Code)
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

func TestContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if contextCancelled(ctx) {
		t.Error("expected false for active context")
	}
	cancel()
	if !contextCancelled(ctx) {
		t.Error("expected true for cancelled context")
	}
}

func TestContextCancelledWithDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Hour))
	defer cancel()
	if !contextCancelled(ctx) {
		t.Error("expected true for expired deadline")
	}
}

func TestNewStreamState(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream, ok := newStreamState(recorder)
	if !ok {
		t.Fatal("expected httptest.ResponseRecorder to support flushing")
	}
	if stream == nil {
		t.Fatal("expected non-nil stream state")
	}
}

func TestStreamStateSendEvent(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream, _ := newStreamState(recorder)
	stream.sendEvent("test", map[string]string{"msg": "hello"})

	body := recorder.Body.String()
	if !strings.Contains(body, "event: test") {
		t.Errorf("expected event 'test', got %q", body)
	}
	if !strings.Contains(body, `"msg":"hello"`) {
		t.Errorf("expected data with msg=hello, got %q", body)
	}
}

func TestStreamStateSendError(t *testing.T) {
	recorder := httptest.NewRecorder()
	stream, _ := newStreamState(recorder)
	stream.sendError("something broke")

	body := recorder.Body.String()
	if !strings.Contains(body, `"message":"something broke"`) {
		t.Errorf("expected error message, got %q", body)
	}
}

func TestGenerateStreamHeaders(t *testing.T) {
	testServer := newTestServer()
	recorder := &flushRecorder{httptest.NewRecorder()}
	request := httptest.NewRequest(http.MethodGet, "/api/generate/stream?input=SELECT+1", nil)
	testServer.ServeHTTP(recorder, request)

	contentType := recorder.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", contentType)
	}
	cacheControl := recorder.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("expected no-cache, got %q", cacheControl)
	}
}

func TestGenerateStreamRequiresInput(t *testing.T) {
	testServer := newTestServer()
	recorder := &flushRecorder{httptest.NewRecorder()}
	testServer.ServeHTTP(recorder, sseRequest("/api/generate/stream"))

	body := recorder.Body.String()
	if !strings.Contains(body, "input (SQL) is required") {
		t.Errorf("expected input error, got %q", body)
	}
}

func TestGenerateStreamRejectsExcessiveRows(t *testing.T) {
	testServer := newTestServer()
	recorder := &flushRecorder{httptest.NewRecorder()}
	testServer.ServeHTTP(recorder, sseRequest("/api/generate/stream?input=SELECT+1&rows=200000"))

	body := recorder.Body.String()
	if !strings.Contains(body, "rows exceeds maximum") {
		t.Errorf("expected rows limit error, got %q", body)
	}
}

func TestJobStoreGetByID(t *testing.T) {
	jobStore := NewJobStore()
	jobStore.Add(&Job{Status: "completed", Tables: 3})
	jobStore.Add(&Job{Status: "completed", Tables: 5})

	job := jobStore.GetByID(1)
	if job == nil || job.Tables != 3 {
		t.Errorf("expected job 1 with 3 tables, got %v", job)
	}

	job = jobStore.GetByID(2)
	if job == nil || job.Tables != 5 {
		t.Errorf("expected job 2 with 5 tables, got %v", job)
	}

	job = jobStore.GetByID(99)
	if job != nil {
		t.Errorf("expected nil for unknown job, got %v", job)
	}
}

func TestGetJobEndpoint(t *testing.T) {
	testServer := newTestServer()

	// Add a job directly
	testServer.jobStore.Add(&Job{
		Status: "completed",
		Tables: 3,
		Format: "csv",
		Data:   []byte("a,b,c\n1,2,3"),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs/1", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var detail jobDetail
	if decodeError := json.NewDecoder(recorder.Body).Decode(&detail); decodeError != nil {
		t.Fatalf("failed to decode: %v", decodeError)
	}

	if detail.Tables != 3 {
		t.Errorf("expected 3 tables, got %d", detail.Tables)
	}
	if detail.Data != "a,b,c\n1,2,3" {
		t.Errorf("expected data 'a,b,c\\n1,2,3', got %q", detail.Data)
	}
}

func TestGetJobEndpointInvalidID(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs/abc", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestGetJobEndpointNotFound(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/jobs/99", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestGenerateStreamPassthroughInTimeoutMiddleware(t *testing.T) {
	testServer := newTestServer()
	recorder := &flushRecorder{httptest.NewRecorder()}
	testServer.ServeHTTP(recorder, sseRequest("/api/generate/stream"))

	body := recorder.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected SSE error event, got %q", body)
	}
}

func TestSecurityHeaders(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	testServer.ServeHTTP(recorder, request)

	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("expected X-Content-Type-Options: nosniff")
	}
	if recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected X-Frame-Options: DENY")
	}
	if recorder.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Error("expected Referrer-Policy: strict-origin-when-cross-origin")
	}
}

func TestRequestIDHeader(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	testServer.ServeHTTP(recorder, request)

	requestID := recorder.Header().Get("X-Request-Id")
	if requestID == "" {
		t.Error("expected X-Request-Id header")
	}
	if len(requestID) != 16 {
		t.Errorf("expected 16-char request ID, got %d: %s", len(requestID), requestID)
	}
}

func TestErrorResponseFormat(t *testing.T) {
	testServer := newTestServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/parse", bytes.NewReader([]byte("{}")))
	request.Header.Set("Content-Type", "application/json")
	testServer.ServeHTTP(recorder, request)

	var decoded struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if decodeError := json.NewDecoder(recorder.Body).Decode(&decoded); decodeError != nil {
		t.Fatalf("failed to decode: %v", decodeError)
	}
	if decoded.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected code BAD_REQUEST, got %q", decoded.Error.Code)
	}
	if decoded.Error.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRateLimitAllowsNormalRequests(t *testing.T) {
	testServer := newTestServer()
	for attempt := 0; attempt < 10; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		testServer.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Errorf("attempt %d: expected 200, got %d", attempt+1, recorder.Code)
		}
	}
}

func TestApiErrorHTTPCodeString(t *testing.T) {
	cases := []struct {
		code     int
		expected string
	}{
		{http.StatusBadRequest, "BAD_REQUEST"},
		{http.StatusNotFound, "NOT_FOUND"},
		{http.StatusInternalServerError, "INTERNAL_ERROR"},
		{http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE"},
		{http.StatusTooManyRequests, "RATE_LIMITED"},
		{http.StatusRequestTimeout, "TIMEOUT"},
		{http.StatusUnprocessableEntity, "UNPROCESSABLE"},
		{418, "HTTP_418"},
	}
	for _, testCase := range cases {
		result := httpCodeString(testCase.code)
		if result != testCase.expected {
			t.Errorf("httpCodeString(%d) = %q, expected %q", testCase.code, result, testCase.expected)
		}
	}
}

func TestDecodeJSONBodyRejectsUnknownFields(t *testing.T) {
	decoder := json.NewDecoder(strings.NewReader(`{"sql":"CREATE TABLE t (id INT)","extra_field":"should fail"}`))
	var target struct {
		SQL string `json:"sql"`
	}
	decoder.DisallowUnknownFields()
	if decodeError := decoder.Decode(&target); decodeError == nil {
		t.Error("expected error for unknown fields")
	}
}

func TestServerStartTimeSet(t *testing.T) {
	if serverStartTime.IsZero() {
		t.Error("expected serverStartTime to be set")
	}
}

func TestNewFlushRecorder(t *testing.T) {
	recorder := &flushRecorder{httptest.NewRecorder()}
	recorder.Flush()
	// Should not panic - flush on a basic recorder is a no-op
}

func TestRateLimiterAllow(t *testing.T) {
	limiter := newRateLimiter(3, 1*time.Minute)
	if !limiter.allow("test-client") {
		t.Error("expected first request to be allowed")
	}
	if !limiter.allow("test-client") {
		t.Error("expected second request to be allowed")
	}
	if !limiter.allow("test-client") {
		t.Error("expected third request to be allowed")
	}
	if limiter.allow("test-client") {
		t.Error("expected fourth request to be denied")
	}
	// Different client should be allowed
	if !limiter.allow("other-client") {
		t.Error("expected different client to be allowed")
	}
}
