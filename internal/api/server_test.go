package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func request(s *Server, method, path, body, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Options{Dir: t.TempDir(), APIKey: "test-key", MaxJobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestAuthenticationAndValidation(t *testing.T) {
	s := newTestServer(t)
	for _, tc := range []struct {
		method, path, body, key string
		status                  int
	}{
		{"GET", "/healthz", "", "", 200},
		{"GET", "/api/v1/benchmarks", "", "", 401},
		{"GET", "/api/v1/benchmarks", "", "bad-key", 401},
		{"GET", "/api/v1/benchmarks", "", "test-key", 200},
		{"POST", "/api/v1/benchmarks", `{}`, "test-key", 400},
		{"POST", "/api/v1/benchmarks", `{"unknown":1}`, "test-key", 400},
		{"POST", "/api/v1/benchmarks", `{} {}`, "test-key", 400},
		{"POST", "/api/v1/benchmarks", `{"dataset":{"samples":[{"input":"a"}]},"model":{"type":"http","endpoint":"http://127.0.0.1/private"}}`, "test-key", 400},
		{"POST", "/api/v1/benchmarks", `{"dataset":{"samples":[{"id":"a"},{"id":"a"}]}}`, "test-key", 400},
		{"POST", "/api/v1/benchmarks", `{"dataset":{"samples":[{"input":"a"}]},"benchmark":{"concurrency":65}}`, "test-key", 400},
		{"GET", "/api/v1/benchmarks/not-an-id", "", "test-key", 404},
	} {
		w := request(s, tc.method, tc.path, tc.body, tc.key)
		if w.Code != tc.status {
			t.Errorf("%s %s: got %d want %d: %s", tc.method, tc.path, w.Code, tc.status, w.Body.String())
		}
	}
}

func TestBenchmarkLifecycleAndPersistence(t *testing.T) {
	s := newTestServer(t)
	w := request(s, "POST", "/api/v1/benchmarks", `{"model":{"type":"mock"},"dataset":{"name":"test","samples":[{"input":"hello","expected":"hello"},{"input":"second"}]},"benchmark":{"limit":1}}`, "test-key")
	if w.Code != 202 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created Job
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	var done Job
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		w = request(s, "GET", "/api/v1/benchmarks/"+created.ID, "", "test-key")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &done); err != nil {
			t.Fatal(err)
		}
		if done.Status != "running" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if done.Status != "completed" || done.Result == nil {
		t.Fatalf("unexpected job: %+v", done)
	}
	if done.Result.Summary.TotalRequests != 1 || done.Result.Quality["exact_match"] != 1 {
		t.Fatalf("unexpected result: %+v", done.Result)
	}
	w = request(s, "POST", "/api/v1/comparisons", `{"baseline":"`+created.ID+`","candidate":"`+created.ID+`"}`, "test-key")
	if w.Code != 200 {
		t.Fatalf("compare: %s", w.Body.String())
	}
	w = request(s, "POST", "/api/v1/benchmarks/"+created.ID+"/cancel", "", "test-key")
	if w.Code != 409 {
		t.Fatalf("terminal cancel: %d", w.Code)
	}
	s.Close()
	restarted, err := New(s.opts)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	w = request(restarted, "GET", "/api/v1/benchmarks/"+created.ID, "", "test-key")
	if w.Code != 200 {
		t.Fatalf("persisted task: %s", w.Body.String())
	}
}

func TestCancellationAndCapacity(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer upstream.Close()
	defer close(release)
	s := newTestServer(t)
	s.opts.AllowedHosts = []string{"127.0.0.1"}
	body := `{"model":{"type":"http","endpoint":"` + upstream.URL + `"},"dataset":{"samples":[{"input":"hello"}]}}`
	w := request(s, "POST", "/api/v1/benchmarks", body, "test-key")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var job Job
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream not called")
	}
	w = request(s, "POST", "/api/v1/benchmarks", body, "test-key")
	if w.Code != 429 {
		t.Fatalf("capacity: %d", w.Code)
	}
	w = request(s, "POST", "/api/v1/benchmarks/"+job.ID+"/cancel", "", "test-key")
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	s.Close()
	w = request(s, "GET", "/api/v1/benchmarks/"+job.ID, "", "test-key")
	if err := json.Unmarshal(w.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Status != "canceled" {
		t.Fatalf("status: %s", job.Status)
	}
}

func TestRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	opts := Options{Dir: dir, APIKey: "test-key"}
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	job := Job{ID: "benchmark-0123456789abcdef0123456789abcdef", Status: "running"}
	if err := s.save(job); err != nil {
		t.Fatal(err)
	}
	s.Close()
	restarted, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	recovered, err := restarted.load(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "interrupted" {
		t.Fatal(recovered.Status)
	}
	if _, err := restarted.load("../../etc/passwd"); !os.IsNotExist(err) {
		t.Fatalf("unsafe ID accepted: %v", err)
	}
}
