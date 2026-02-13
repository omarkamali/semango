package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/omarkamali/semango/internal/pipeline"
)

func TestHandleIndexingStatus_nil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{}
	router := gin.New()
	router.GET("/api/v1/indexing-status", s.handleIndexingStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/indexing-status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if active, ok := body["active"]; !ok || active != false {
		t.Errorf("expected active=false, got %v", body)
	}
}

func TestHandleIndexingStatus_active(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var status pipeline.IndexingStatus
	status.Active.Store(true)
	status.FilesTotal.Store(50)
	status.FilesDone.Store(25)

	s := &Server{indexingStatus: &status}
	router := gin.New()
	router.GET("/api/v1/indexing-status", s.handleIndexingStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/indexing-status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var snap pipeline.IndexingStatusSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if !snap.Active {
		t.Error("expected active")
	}
	if snap.FilesTotal != 50 {
		t.Errorf("expected 50 total, got %d", snap.FilesTotal)
	}
	if snap.FilesDone != 25 {
		t.Errorf("expected 25 done, got %d", snap.FilesDone)
	}
	if snap.Progress < 0.49 || snap.Progress > 0.51 {
		t.Errorf("expected ~0.5 progress, got %f", snap.Progress)
	}
}
