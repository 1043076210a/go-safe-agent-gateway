package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegister_WhenDemoRoutesRequested_ShouldServeEmbeddedAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r)

	page := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	r.ServeHTTP(page, req)
	if page.Code != http.StatusOK {
		t.Fatalf("demo page status = %d", page.Code)
	}
	if !strings.Contains(page.Body.String(), "Go Safe Agent Gateway") {
		t.Fatalf("demo page did not contain expected title")
	}

	asset := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/demo/app.js", nil)
	r.ServeHTTP(asset, req)
	if asset.Code != http.StatusOK {
		t.Fatalf("demo asset status = %d", asset.Code)
	}
	if !strings.Contains(asset.Body.String(), "searchKnowledgeBase") {
		t.Fatalf("demo asset did not contain expected script")
	}
}
