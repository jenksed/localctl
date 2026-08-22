package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestWeb(t *testing.T) (*localWeb, modelArtifact) {
	t.Helper()
	home := useTempLocalCTLHome(t)
	model := seedTestModel(t, home, "http-Q4_K_M.gguf")
	web, err := newLocalWeb(newLocalApplication())
	if err != nil {
		t.Fatal(err)
	}
	return web, model
}

func localRequest(method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Host = "127.0.0.1:7331"
	return req
}

func TestWebListenAddressIsAlwaysLoopback(t *testing.T) {
	address, err := webListenAddress(7331)
	if err != nil {
		t.Fatal(err)
	}
	if address != "127.0.0.1:7331" {
		t.Fatalf("address=%s", address)
	}
	if _, err := webListenAddress(70000); err == nil {
		t.Fatal("expected invalid port error")
	}
}

func TestFirstLaunchBootstrapUsesRealLocalState(t *testing.T) {
	web, _ := newTestWeb(t)
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, localRequest(http.MethodGet, "http://127.0.0.1:7331/api/v1/bootstrap", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var result bootstrapProjection
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.FirstLaunch || result.Machine.Models != 1 || result.Machine.EvidenceRuns != 0 {
		t.Fatalf("bootstrap=%#v", result)
	}
	if result.CSRFToken == "" {
		t.Fatal("missing csrf token")
	}
}

func TestHTTPRejectsInvalidHostAndCrossOriginMutation(t *testing.T) {
	web, _ := newTestWeb(t)

	invalidHost := httptest.NewRequest(http.MethodGet, "http://evil.invalid/api/v1/bootstrap", nil)
	invalidHost.Host = "evil.invalid"
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, invalidHost)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("invalid Host status=%d", rr.Code)
	}

	cross := localRequest(http.MethodPost, "http://127.0.0.1:7331/api/v1/index/rebuild", []byte("{}"))
	cross.Header.Set("X-LocalCTL-CSRF", web.csrf)
	cross.Header.Set("Origin", "http://evil.invalid")
	rr = httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, cross)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPMutationRequiresCSRFAndAllowsSameOrigin(t *testing.T) {
	web, _ := newTestWeb(t)
	without := localRequest(http.MethodPost, "http://127.0.0.1:7331/api/v1/index/rebuild", []byte("{}"))
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, without)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("without csrf status=%d", rr.Code)
	}

	with := localRequest(http.MethodPost, "http://127.0.0.1:7331/api/v1/index/rebuild", []byte("{}"))
	with.Header.Set("X-LocalCTL-CSRF", web.csrf)
	with.Header.Set("Origin", "http://127.0.0.1:7331")
	rr = httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, with)
	if rr.Code != http.StatusOK {
		t.Fatalf("same-origin mutation status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPHasNoArbitraryCommandExecutionRoute(t *testing.T) {
	web, _ := newTestWeb(t)
	req := localRequest(http.MethodPost, "http://127.0.0.1:7331/api/v1/execute", []byte(`{"command":"rm -rf /"}`))
	req.Header.Set("X-LocalCTL-CSRF", web.csrf)
	req.Header.Set("Origin", "http://127.0.0.1:7331")
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected execute route status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPJobInputIsBoundedAndUnknownFieldsAreRejected(t *testing.T) {
	web, model := newTestWeb(t)
	body := []byte(`{"mission_id":"developer","model_id":"` + model.ID + `","shell":"whoami"}`)
	req := localRequest(http.MethodPost, "http://127.0.0.1:7331/api/v1/jobs", body)
	req.Header.Set("X-LocalCTL-CSRF", web.csrf)
	req.Header.Set("Origin", "http://127.0.0.1:7331")
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "unknown field") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPServesEmbeddedFrontendWithSecurityHeaders(t *testing.T) {
	web, _ := newTestWeb(t)
	rr := httptest.NewRecorder()
	web.Handler().ServeHTTP(rr, localRequest(http.MethodGet, "http://127.0.0.1:7331/", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "LocalCTL") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Content-Security-Policy") == "" || rr.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers missing: %#v", rr.Header())
	}
}
