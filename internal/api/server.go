// Package api exposes the benchmark engine as an asynchronous HTTP service.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ayu-v0/ModelPilot/internal/benchmark"
	"github.com/ayu-v0/ModelPilot/internal/config"
	"github.com/ayu-v0/ModelPilot/internal/domain"
	"github.com/ayu-v0/ModelPilot/pkg/model"
)

const maxBody = 8 << 20

var validID = regexp.MustCompile(`^benchmark-[a-f0-9]{32}$`)

// Options are server-side settings, not controlled by API callers.
type Options struct {
	Dir          string
	APIKey       string
	AllowedHosts []string
	MaxJobs      int
	JobTimeout   time.Duration
}

type Job struct {
	ID         string                  `json:"id"`
	Status     string                  `json:"status"`
	CreatedAt  string                  `json:"created_at"`
	FinishedAt string                  `json:"finished_at,omitempty"`
	Error      string                  `json:"error,omitempty"`
	Result     *domain.BenchmarkResult `json:"result,omitempty"`
}

type CreateRequest struct {
	Name      string                 `json:"name"`
	Model     config.ModelConfig     `json:"model"`
	Dataset   domain.Dataset         `json:"dataset"`
	Benchmark config.BenchmarkConfig `json:"benchmark"`
}

type Server struct {
	opts    Options
	mu      sync.Mutex
	active  map[string]context.CancelFunc
	wg      sync.WaitGroup
	closed  bool
	handler http.Handler
}

func New(opts Options) (*Server, error) {
	if opts.APIKey == "" {
		return nil, errors.New("MODELPILOT_API_KEY is required")
	}
	if opts.Dir == "" {
		opts.Dir = "./data/jobs"
	}
	if opts.MaxJobs <= 0 {
		opts.MaxJobs = 4
	}
	if opts.JobTimeout <= 0 {
		opts.JobTimeout = 30 * time.Minute
	}
	if err := os.MkdirAll(opts.Dir, 0700); err != nil {
		return nil, err
	}
	s := &Server{opts: opts, active: make(map[string]context.CancelFunc)}
	// No credentials or input configuration are persisted. An interrupted run
	// is never silently resumed with incomplete credentials after a restart.
	files, err := filepath.Glob(filepath.Join(opts.Dir, "benchmark-*.json"))
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		if !validID.MatchString(job.ID) {
			return nil, fmt.Errorf("invalid stored job ID in %s", file)
		}
		if job.Status == "running" {
			job.Status, job.Error = "interrupted", "service restarted before task completed"
			job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := s.save(job); err != nil {
				return nil, err
			}
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /api/v1/benchmarks", s.create)
	mux.HandleFunc("GET /api/v1/benchmarks", s.list)
	mux.HandleFunc("GET /api/v1/benchmarks/{id}", s.get)
	mux.HandleFunc("POST /api/v1/benchmarks/{id}/cancel", s.cancel)
	mux.HandleFunc("POST /api/v1/comparisons", s.compare)
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/healthz" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+opts.APIKey)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			fail(w, 401, "unauthorized")
			return
		}
		mux.ServeHTTP(w, r)
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// Close prevents new jobs and cancels and waits for all in-flight runs.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	for _, cancel := range s.active {
		cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			fail(w, 413, "request body exceeds 8 MiB")
		} else {
			fail(w, 400, "invalid JSON: "+err.Error())
		}
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		fail(w, 400, "expected exactly one JSON object")
		return false
	}
	return true
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if !decode(w, r, &req) {
		return
	}
	selected, err := s.validate(&req)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		fail(w, 500, "unable to create task")
		return
	}
	id := "benchmark-" + hex.EncodeToString(random[:])
	job := Job{ID: id, Status: "running", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		fail(w, 503, "service shutting down")
		return
	}
	if len(s.active) >= s.opts.MaxJobs {
		fail(w, 429, "too many active tasks; retry later")
		return
	}
	if err := s.save(job); err != nil {
		fail(w, 500, "unable to persist task")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.JobTimeout)
	s.active[id] = cancel
	s.wg.Add(1)
	go s.run(ctx, cancel, job, req, selected)
	w.Header().Set("Location", "/api/v1/benchmarks/"+id)
	respond(w, 202, job)
}

func (s *Server) validate(req *CreateRequest) (domain.Model, error) {
	if len(req.Dataset.Samples) == 0 || len(req.Dataset.Samples) > 10000 {
		return nil, errors.New("dataset must contain 1 to 10000 samples")
	}
	seen := make(map[string]bool)
	for i := range req.Dataset.Samples {
		sample := &req.Dataset.Samples[i]
		if sample.ID == "" {
			sample.ID = fmt.Sprintf("sample-%d", i+1)
		}
		if seen[sample.ID] {
			return nil, errors.New("sample IDs must be unique")
		}
		seen[sample.ID] = true
	}
	b := &req.Benchmark
	if b.Concurrency < 0 || b.Concurrency > 64 || b.TimeoutSeconds < 0 || b.TimeoutSeconds > 300 || b.Limit < 0 {
		return nil, errors.New("concurrency must be 0..64, timeout_seconds 0..300, limit nonnegative")
	}
	if b.Concurrency == 0 {
		b.Concurrency = 1
	}
	if b.TimeoutSeconds == 0 {
		b.TimeoutSeconds = 30
	}
	if b.Limit > 0 && b.Limit < len(req.Dataset.Samples) {
		req.Dataset.Samples = req.Dataset.Samples[:b.Limit]
	}
	if req.Dataset.Name == "" {
		req.Dataset.Name = "api-dataset"
	}
	m := req.Model
	if m.Temperature < 0 || m.Temperature > 2 || m.MaxTokens < 0 || m.MaxTokens > 32768 {
		return nil, errors.New("temperature must be 0..2 and max_tokens 0..32768")
	}
	if m.MaxTokens == 0 {
		m.MaxTokens = 4096
	}
	if m.Type == "" || m.Type == "mock" {
		return model.Mock{ModelName: m.Name}, nil
	}
	if m.Type != "http" && m.Type != "openai" {
		return nil, errors.New("model.type must be mock, http or openai")
	}
	u, err := url.Parse(m.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("endpoint must be an http(s) URL without userinfo or fragment")
	}
	allowed := false
	for _, host := range s.opts.AllowedHosts {
		if strings.EqualFold(strings.TrimSpace(host), u.Hostname()) {
			allowed = true
		}
	}
	if !allowed {
		return nil, errors.New("endpoint hostname is not in MODELPILOT_ALLOWED_HOSTS")
	}
	// Do not follow redirects to destinations outside the configured allowlist.
	client := &http.Client{Timeout: time.Duration(b.TimeoutSeconds) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if m.Type == "http" {
		return model.HTTP{ModelName: m.Name, Endpoint: m.Endpoint, Token: m.Token, Client: client}, nil
	}
	if m.Name == "" {
		return nil, errors.New("model.name is required for openai")
	}
	return model.OpenAICompatible{ModelName: m.Name, Endpoint: m.Endpoint, Token: m.Token, Temperature: m.Temperature, MaxTokens: m.MaxTokens, Client: client}, nil
}

func (s *Server) run(ctx context.Context, cancel context.CancelFunc, job Job, req CreateRequest, selected domain.Model) {
	defer s.wg.Done()
	defer cancel()
	task := domain.BenchmarkTask{ID: job.ID, Name: req.Name, Model: selected.Name(), Dataset: req.Dataset.Name, Concurrency: req.Benchmark.Concurrency, TimeoutSeconds: req.Benchmark.TimeoutSeconds, QualityMetrics: []string{"exact_match"}}
	result, err := (benchmark.Runner{Model: selected}).Run(ctx, task, req.Dataset)
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = "completed"
	if err != nil {
		job.Status, job.Error = "failed", "benchmark execution failed"
	} else {
		job.Result = &result
	}
	if ctx.Err() != nil {
		job.Status = "canceled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			job.Status = "timed_out"
		}
	}
	job.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	// Model error messages may echo credentials supplied to an upstream service.
	if job.Result != nil && req.Model.Token != "" {
		for i := range job.Result.Requests {
			job.Result.Requests[i].Error = strings.ReplaceAll(job.Result.Requests[i].Error, req.Model.Token, "[REDACTED]")
		}
	}
	if err := s.save(job); err != nil {
		// Leave the on-disk running record for restart recovery rather than
		// pretending the result was successfully persisted.
		fmt.Fprintf(os.Stderr, "persist task %s: %v\n", job.ID, err)
	}
	delete(s.active, job.ID)
}

func (s *Server) save(job Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.opts.Dir, ".job-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(s.opts.Dir, job.ID+".json"))
}

func (s *Server) load(id string) (Job, error) {
	if !validID.MatchString(id) {
		return Job{}, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(s.opts.Dir, id+".json"))
	if err != nil {
		return Job{}, err
	}
	var job Job
	err = json.Unmarshal(data, &job)
	return job, err
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.load(r.PathValue("id"))
	if err != nil {
		storageError(w, err)
		return
	}
	respond(w, 200, job)
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := filepath.Glob(filepath.Join(s.opts.Dir, "benchmark-*.json"))
	if err != nil {
		fail(w, 500, "unable to list tasks")
		return
	}
	jobs := make([]Job, 0, len(files))
	for _, file := range files {
		job, err := s.load(strings.TrimSuffix(filepath.Base(file), ".json"))
		if err != nil {
			storageError(w, err)
			return
		}
		job.Result = nil
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].CreatedAt > jobs[j].CreatedAt })
	respond(w, 200, map[string]any{"items": jobs})
}

func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := r.PathValue("id")
	if _, err := s.load(id); err != nil {
		storageError(w, err)
		return
	}
	cancel, ok := s.active[id]
	if !ok {
		fail(w, 409, "task is already terminal")
		return
	}
	cancel()
	respond(w, 202, map[string]string{"id": id, "status": "cancel_requested"})
}

func (s *Server) compare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Baseline   string `json:"baseline"`
		Candidate  string `json:"candidate"`
		Thresholds *struct {
			Latency    float64 `json:"latency"`
			Throughput float64 `json:"throughput"`
			Success    float64 `json:"success"`
			Quality    float64 `json:"quality"`
		} `json:"thresholds"`
	}
	if !decode(w, r, &req) {
		return
	}
	cfg := benchmark.DefaultComparisonConfig()
	if t := req.Thresholds; t != nil {
		if t.Latency < 0 || t.Throughput < 0 || t.Success < 0 || t.Quality < 0 {
			fail(w, 400, "thresholds must be nonnegative")
			return
		}
		cfg = benchmark.ComparisonConfig{LatencyThresholdPercent: t.Latency, ThroughputThresholdPercent: t.Throughput, SuccessThresholdPercent: t.Success, QualityThresholdPercent: t.Quality}
	}
	s.mu.Lock()
	baseline, err := s.load(req.Baseline)
	if err != nil {
		s.mu.Unlock()
		storageError(w, err)
		return
	}
	candidate, err := s.load(req.Candidate)
	s.mu.Unlock()
	if err != nil {
		storageError(w, err)
		return
	}
	if baseline.Status != "completed" || candidate.Status != "completed" || baseline.Result == nil || candidate.Result == nil {
		fail(w, 409, "both tasks must be completed")
		return
	}
	respond(w, 200, benchmark.Compare(*baseline.Result, *candidate.Result, cfg))
}

func storageError(w http.ResponseWriter, err error) {
	if errors.Is(err, os.ErrNotExist) {
		fail(w, 404, "task not found")
	} else {
		fail(w, 500, "unable to read task")
	}
}
func fail(w http.ResponseWriter, status int, message string) {
	respond(w, status, map[string]string{"error": message})
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
