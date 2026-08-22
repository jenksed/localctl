package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

//go:embed web/dist/*.js web/src/index.html web/src/styles.css
var webAssets embed.FS

type localWeb struct {
	app    *localApplication
	jobs   *jobManager
	csrf   string
	assets fs.FS
}

type bootstrapProjection struct {
	Version     string            `json:"version"`
	CSRFToken   string            `json:"csrf_token"`
	Machine     machineInspection `json:"machine"`
	FirstLaunch bool              `json:"first_launch"`
	Navigation  []string          `json:"navigation"`
	Authority   []string          `json:"authority"`
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newLocalWeb(app *localApplication) (*localWeb, error) {
	jobs, err := newJobManager(app)
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &localWeb{app: app, jobs: jobs, csrf: token, assets: webAssets}, nil
}

func webListenAddress(port int) (string, error) {
	if port < 0 || port > 65535 {
		return "", fmt.Errorf("invalid port %d", port)
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func runWebLab(args []string, stdout, stderr io.Writer) int {
	port := 7331
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--port="):
			value := strings.TrimPrefix(arg, "--port=")
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 65535 {
				fmt.Fprintf(stderr, "invalid web port: %s\n", value)
				return 1
			}
			port = parsed
		default:
			fmt.Fprintf(stderr, "unexpected lab web argument: %s\n", arg)
			return 1
		}
	}
	app := newLocalApplication()
	web, err := newLocalWeb(app)
	if err != nil {
		fmt.Fprintf(stderr, "could not initialize web lab: %v\n", err)
		return 1
	}
	address, _ := webListenAddress(port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Fprintf(stderr, "could not listen on %s: %v\n", address, err)
		return 1
	}
	defer listener.Close()
	fmt.Fprintf(stdout, "LocalCTL web lab\nhttp://%s\n\n", address)
	fmt.Fprintln(stdout, "Bound to loopback only. Ctrl-C stops the UI server; durable jobs/evidence remain inspectable on restart.")
	server := &http.Server{Handler: web.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "web lab stopped: %v\n", err)
		return 1
	}
	return 0
}

func (w *localWeb) Handler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.securityHeaders(rw)
		if !validLocalHost(r.Host) {
			writeAPIError(rw, http.StatusForbidden, "invalid_host", "LocalCTL accepts only localhost/127.0.0.1 Host values")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.serveAPI(rw, r)
			return
		}
		w.serveAsset(rw, r)
	})
}

func (w *localWeb) securityHeaders(rw http.ResponseWriter) {
	rw.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	rw.Header().Set("Referrer-Policy", "no-referrer")
	rw.Header().Set("X-Content-Type-Options", "nosniff")
	rw.Header().Set("X-Frame-Options", "DENY")
	rw.Header().Set("Cache-Control", "no-store")
}

func validLocalHost(raw string) bool {
	host := raw
	if split, _, err := net.SplitHostPort(raw); err == nil {
		host = split
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	return host == "127.0.0.1" || host == "localhost"
}

func (w *localWeb) mutationAllowed(r *http.Request) bool {
	if r.Header.Get("X-LocalCTL-CSRF") != w.csrf {
		return false
	}
	if site := strings.ToLower(r.Header.Get("Sec-Fetch-Site")); site == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host) && validLocalHost(parsed.Host)
}

func (w *localWeb) requireMutation(rw http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		return true
	}
	if !w.mutationAllowed(r) {
		writeAPIError(rw, http.StatusForbidden, "mutation_rejected", "mutation requires a valid same-origin request and X-LocalCTL-CSRF token")
		return false
	}
	return true
}

func (w *localWeb) serveAPI(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		writeAPIError(rw, http.StatusMethodNotAllowed, "cors_disabled", "LocalCTL does not expose cross-origin API access")
		return
	}
	if !w.requireMutation(rw, r) {
		return
	}
	profile := r.URL.Query().Get("profile")
	if profile == "" {
		profile = "default"
	}
	path := strings.TrimSuffix(r.URL.Path, "/")

	switch {
	case path == "/api/v1/bootstrap" && r.Method == http.MethodGet:
		machine, err := w.app.InspectMachine(r.Context())
		if err != nil {
			writeInternalError(rw, err)
			return
		}
		writeJSON(rw, http.StatusOK, bootstrapProjection{Version: localctlVersion, CSRFToken: w.csrf, Machine: machine, FirstLaunch: machine.EvidenceRuns == 0, Navigation: []string{"MAP", "EXPLORE", "USE", "HISTORY", "MODELS", "SETTINGS"}, Authority: []string{"observation is durable source evidence", "derived capability is deterministic and versioned", "recommendation is advisory", "presentation does not become truth"}})
	case path == "/api/v1/territory" && r.Method == http.MethodGet:
		value, err := w.app.GetTerritory(r.Context(), profile)
		writeResult(rw, value, err)
	case path == "/api/v1/models" && r.Method == http.MethodGet:
		value, err := w.app.ListModels(r.Context(), profile)
		writeResult(rw, value, err)
	case path == "/api/v1/missions" && r.Method == http.MethodGet:
		value, err := w.app.ListMissions(r.Context())
		writeResult(rw, value, err)
	case strings.HasPrefix(path, "/api/v1/missions/") && strings.HasSuffix(path, "/plan") && r.Method == http.MethodGet:
		missionID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/missions/"), "/plan")
		value, err := w.app.PlanMission(r.Context(), missionID, r.URL.Query().Get("model"), profile)
		writeResult(rw, value, err)
	case path == "/api/v1/scout" && r.Method == http.MethodGet:
		value, err := w.app.GetScoutItems(r.Context(), profile)
		writeResult(rw, value, err)
	case path == "/api/v1/history" && r.Method == http.MethodGet:
		value, err := w.app.History(r.Context(), boundedLimit(r.URL.Query().Get("limit"), 80, 200))
		writeResult(rw, value, err)
	case strings.HasPrefix(path, "/api/v1/runs/") && r.Method == http.MethodGet:
		runID := strings.TrimPrefix(path, "/api/v1/runs/")
		value, err := w.app.GetRun(r.Context(), runID)
		writeResult(rw, value, err)
	case path == "/api/v1/use" && r.Method == http.MethodGet:
		value, err := w.app.ListUseRecommendations(r.Context(), profile)
		writeResult(rw, value, err)
	case strings.HasPrefix(path, "/api/v1/use/") && r.Method == http.MethodGet:
		workload := strings.TrimPrefix(path, "/api/v1/use/")
		value, err := w.app.Recommend(r.Context(), workload, profile)
		writeResult(rw, value, err)
	case path == "/api/v1/settings" && r.Method == http.MethodGet:
		value, err := w.app.Settings(r.Context())
		writeResult(rw, value, err)
	case path == "/api/v1/index/rebuild" && r.Method == http.MethodPost:
		count, err := w.app.RebuildReadIndex(r.Context())
		if err != nil {
			writeInternalError(rw, err)
			return
		}
		writeJSON(rw, http.StatusOK, map[string]interface{}{"records": count, "authority": "rebuilt from durable observation.json source files"})
	case path == "/api/v1/jobs" && r.Method == http.MethodGet:
		value, err := w.jobs.List(boundedLimit(r.URL.Query().Get("limit"), 30, 100))
		writeResult(rw, value, err)
	case path == "/api/v1/jobs" && r.Method == http.MethodPost:
		w.startJob(rw, r)
	case strings.HasPrefix(path, "/api/v1/jobs/") && strings.HasSuffix(path, "/cancel") && r.Method == http.MethodPost:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/jobs/"), "/cancel")
		value, err := w.jobs.Cancel(id)
		writeResult(rw, value, err)
	case strings.HasPrefix(path, "/api/v1/jobs/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/api/v1/jobs/")
		value, err := w.jobs.Get(id)
		writeResult(rw, value, err)
	case path == "/api/v1/events" && r.Method == http.MethodGet:
		w.serveEvents(rw, r)
	default:
		writeAPIError(rw, http.StatusNotFound, "not_found", "API route not found")
	}
}

func boundedLimit(raw string, fallback, max int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func (w *localWeb) startJob(rw http.ResponseWriter, r *http.Request) {
	var request struct {
		MissionID string `json:"mission_id"`
		ModelID   string `json:"model_id"`
		ProfileID string `json:"profile_id"`
	}
	if err := decodeBoundedJSON(rw, r, &request); err != nil {
		return
	}
	if request.MissionID == "" || request.ModelID == "" {
		writeAPIError(rw, http.StatusBadRequest, "invalid_job", "mission_id and model_id are required")
		return
	}
	if request.ProfileID == "" {
		request.ProfileID = "default"
	}
	record, created, err := w.jobs.StartMission(r.Context(), request.MissionID, request.ModelID, request.ProfileID)
	if err != nil {
		status := http.StatusConflict
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "no model matched") {
			status = http.StatusBadRequest
		}
		writeAPIError(rw, status, "job_not_started", err.Error())
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusAccepted
	}
	writeJSON(rw, status, map[string]interface{}{"created": created, "job": record})
}

func decodeBoundedJSON(rw http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(rw, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(rw, http.StatusBadRequest, "invalid_json", err.Error())
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		writeAPIError(rw, http.StatusBadRequest, "invalid_json", "request must contain one JSON object")
		return fmt.Errorf("extra JSON content")
	}
	return nil
}

func (w *localWeb) serveEvents(rw http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if !safeLocalID(jobID) {
		writeAPIError(rw, http.StatusBadRequest, "invalid_job_id", "job_id is required")
		return
	}
	flusher, ok := rw.(http.Flusher)
	if !ok {
		writeAPIError(rw, http.StatusInternalServerError, "stream_unsupported", "HTTP streaming is unavailable")
		return
	}
	after := boundedLimit(r.URL.Query().Get("after"), 0, 1000000)
	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("Cache-Control", "no-store")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		record, err := w.jobs.Get(jobID)
		if err != nil {
			fmt.Fprintf(rw, "event: error\ndata: %s\n\n", mustJSON(map[string]string{"error": err.Error()}))
			flusher.Flush()
			return
		}
		for _, event := range record.Events {
			if event.Sequence <= after {
				continue
			}
			fmt.Fprintf(rw, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, mustJSON(event))
			after = event.Sequence
		}
		flusher.Flush()
		if terminalJobState(record.State) {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func mustJSON(value interface{}) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func writeResult(rw http.ResponseWriter, value interface{}, err error) {
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeAPIError(rw, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if strings.Contains(err.Error(), "invalid") || strings.Contains(err.Error(), "ambiguous") || strings.Contains(err.Error(), "no model matched") {
			writeAPIError(rw, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeInternalError(rw, err)
		return
	}
	writeJSON(rw, http.StatusOK, value)
}

func writeInternalError(rw http.ResponseWriter, err error) {
	writeAPIError(rw, http.StatusInternalServerError, "internal_error", err.Error())
}

func writeAPIError(rw http.ResponseWriter, status int, code, message string) {
	writeJSON(rw, status, map[string]interface{}{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(rw http.ResponseWriter, status int, value interface{}) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(value)
}

func (w *localWeb) serveAsset(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if !safeAssetPath(path) {
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	assetPath := "web/dist/" + path
	if path == "index.html" || path == "styles.css" {
		assetPath = "web/src/" + path
	}
	data, err := fs.ReadFile(w.assets, assetPath)
	if err != nil {
		data, err = fs.ReadFile(w.assets, "web/src/index.html")
		path = "index.html"
	}
	if err != nil {
		writeAPIError(rw, http.StatusInternalServerError, "assets_missing", err.Error())
		return
	}
	switch {
	case strings.HasSuffix(path, ".html"):
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		rw.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		rw.Header().Set("Content-Type", "text/css; charset=utf-8")
	}
	_, _ = rw.Write(data)
}

func safeAssetPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
