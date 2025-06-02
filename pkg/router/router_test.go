package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/code-sigs/go-box/internal/handler"
	"github.com/code-sigs/go-box/pkg/rpcerror"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type TestRequest struct {
	Name string `json:"name"`
}

type TestResponse struct {
	Greet string `json:"greet"`
}

func mockGRPCFunc(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	return &TestResponse{Greet: "Hello, " + req.Name}, nil
}

func mockGRPCFuncError(ctx context.Context, req *TestRequest) (*TestResponse, error) {
	return &TestResponse{Greet: "Hello, " + req.Name}, rpcerror.WrapCode(5100, "mock grpc error 5000")
}

func TestGenericGRPCHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/test", handler.GenericGRPCHandler(mockGRPCFunc, handler.DefaultContextInjector))

	body, _ := json.Marshal(TestRequest{Name: "GoBox"})
	req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("Response body: %s", w.Body.String()) // 打印响应体

	assert.Equal(t, http.StatusOK, w.Code)
	var resp handler.StandardResponse[*TestResponse]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, int32(0), resp.Code)
	assert.Equal(t, "ok", resp.Message)
	assert.Equal(t, "Hello, GoBox", resp.Data.Greet)
}

func TestGenericGRPCHandler_BadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/test", handler.GenericGRPCHandler(mockGRPCFunc, handler.DefaultContextInjector))

	req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer([]byte(`invalid json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("Response body: %s", w.Body.String()) // 打印响应体

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp handler.StandardResponse[any]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, int32(400), resp.Code)
}

func TestGenericGRPCHandler_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/test", handler.GenericGRPCHandler(mockGRPCFuncError, handler.DefaultContextInjector))

	body, _ := json.Marshal(TestRequest{Name: "GoBox"})
	req, _ := http.NewRequest("POST", "/test", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	t.Logf("Response body: %s", w.Body.String()) // 打印响应体

	assert.Equal(t, http.StatusOK, w.Code)
	var resp handler.StandardResponse[any]
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, int32(5100), resp.Code)
	assert.Contains(t, resp.Message, "mock grpc error")
}
