package wasm

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"github.com/cardinalby/depo/demo/internal/domain"
)

type usecase interface {
	Graph() domain.Graph
	StartAll() error
	StopAll() error
	Reset(shutDownOnNilRunResult bool)
	UpdateComponent(compID uint64, startErr string, delay time.Duration) error
	StopComponent(compID uint64, withErr bool) error
}

type MethodRequest struct {
	Method string          `json:"method"`
	Body   json.RawMessage `json:"body"`
}

type MethodResponse struct {
	Error string `json:"error"`
	Body  any    `json:"body"`
}

func makeError(message string) interface{} {
	return js.Global().Get("Error").New(message)
}

type wasmHandler struct {
	usecase usecase
}

func NewWasmHandler(usecase usecase) js.Func {
	h := &wasmHandler{
		usecase: usecase,
	}
	return h.jsHandleFunc()
}

func (h *wasmHandler) jsHandleFunc() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		var resp MethodResponse
		if len(args) != 1 {
			resp.Error = "single argument expected"
			bytes, _ := json.Marshal(&resp)
			return js.ValueOf(string(bytes))
		}
		var req MethodRequest
		if err := json.Unmarshal([]byte(args[0].String()), &req); err != nil {
			resp.Error = "invalid request format: " + err.Error()
			bytes, _ := json.Marshal(&resp)
			return js.ValueOf(string(bytes))
		}
		body, err := h.handleRequest(req)
		resp.Body = body
		if err != nil {
			resp.Error = err.Error()
		}
		bytes, _ := json.Marshal(&resp)
		return js.ValueOf(string(bytes))
	})
}

func (h *wasmHandler) makeError(message string) any {
	return js.Global().Get("Error").New(message)
}

func (h *wasmHandler) handleRequest(req MethodRequest) (any, error) {
	switch req.Method {
	case "graph":
		return h.usecase.Graph(), nil
	case "startAll":
		return nil, h.usecase.StartAll()
	case "stopAll":
		return nil, h.usecase.StopAll()
	case "reset":
		var resetReq struct {
			ShutDownOnNilRunResult bool `json:"shut_down_on_nil_run_result"`
		}
		if err := json.Unmarshal(req.Body, &resetReq); err != nil {
			return nil, fmt.Errorf("invalid request body: %w", err)
		}
		h.usecase.Reset(resetReq.ShutDownOnNilRunResult)
		return nil, nil
	case "updateComponent":
		var updateReq struct {
			ComponentID uint64 `json:"component_id"`
			DelayMs     int    `json:"delay_ms"`
			StartError  string `json:"start_error"`
		}
		if err := json.Unmarshal(req.Body, &updateReq); err != nil {
			return nil, fmt.Errorf("invalid request body: %w", err)
		}
		if err := h.usecase.UpdateComponent(
			updateReq.ComponentID,
			updateReq.StartError,
			time.Duration(updateReq.DelayMs)*time.Millisecond,
		); err != nil {
			return nil, fmt.Errorf("failed to update component: %w", err)
		}
		return nil, nil
	case "stopComponent":
		var stopReq struct {
			ComponentID uint64 `json:"component_id"`
			WithError   bool   `json:"with_error"`
		}
		if err := json.Unmarshal(req.Body, &stopReq); err != nil {
			return nil, fmt.Errorf("invalid request body: %w", err)
		}
		if err := h.usecase.StopComponent(stopReq.ComponentID, stopReq.WithError); err != nil {
			return nil, fmt.Errorf("failed to stop component: %w", err)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown method: %s", req.Method)
	}
}
