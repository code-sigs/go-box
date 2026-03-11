package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		t.Fatalf("fileName should be present, response: %s", resp.Body.String())
	}
	if fileName != "" {
		t.Fatalf("expected empty fileName, got %#v", fileName)
	}
}

func TestNormalizeResponseData_ProtoScalarTypesStayConsistent(t *testing.T) {
	cases := []struct {
		name        string
		message     any
		expectType  reflect.Type
		expectValue any
	}{
		{name: "bool", message: wrapperspb.Bool(true), expectType: reflect.TypeOf(true), expectValue: true},
		{name: "int32", message: wrapperspb.Int32(12), expectType: reflect.TypeOf(int32(0)), expectValue: int32(12)},
		{name: "int64", message: wrapperspb.Int64(34), expectType: reflect.TypeOf(int64(0)), expectValue: int64(34)},
		{name: "uint32", message: wrapperspb.UInt32(56), expectType: reflect.TypeOf(uint32(0)), expectValue: uint32(56)},
		{name: "uint64", message: wrapperspb.UInt64(78), expectType: reflect.TypeOf(uint64(0)), expectValue: uint64(78)},
		{name: "float", message: wrapperspb.Float(1.25), expectType: reflect.TypeOf(float32(0)), expectValue: float32(1.25)},
		{name: "double", message: wrapperspb.Double(2.5), expectType: reflect.TypeOf(float64(0)), expectValue: float64(2.5)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := normalizeResponseData(tc.message)
			if err != nil {
				t.Fatalf("normalizeResponseData: %v", err)
			}
			obj, ok := data.(map[string]any)
			if !ok {
				t.Fatalf("expected object response, got %#v", data)
			}
			value := obj["value"]
			if reflect.TypeOf(value) != tc.expectType {
				t.Fatalf("unexpected type: got %v want %v value=%#v", reflect.TypeOf(value), tc.expectType, value)
			}
			if !reflect.DeepEqual(value, tc.expectValue) {
				t.Fatalf("unexpected value: got %#v want %#v", value, tc.expectValue)
			}
		})
	}
}
