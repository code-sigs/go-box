package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
)

type testRequest struct {
	Name string `json:"name"`
}

func TestGenericGRPCHandler_EmitUnpopulatedForProtoResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := GenericGRPCHandler(func(ctx context.Context, req *testRequest) (*sourcecontextpb.SourceContext, error) {
		if req.Name != "router" {
			t.Fatalf("unexpected request: %#v", req)
		}
		return &sourcecontextpb.SourceContext{}, nil
	}, nil)

	engine := gin.New()
	engine.POST("/test", handler)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"name":"router"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d, body: %s", resp.Code, resp.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data missing or invalid: %#v", payload["data"])
	}
	fileName, exists := data["fileName"]
	if !exists {
		t.Fatalf("fileName should be present due to EmitUnpopulated, response: %s", resp.Body.String())
	}
	if fileName != "" {
		t.Fatalf("expected empty fileName, got %#v", fileName)
	}
}
