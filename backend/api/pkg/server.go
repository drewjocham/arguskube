package pkg

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
)

type APIRequest struct {
	Args []interface{} `json:"args"`
}

type APIResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS for dev and SaaS mode
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/")
	methodName := path

	method := reflect.ValueOf(a).MethodByName(methodName)
	if !method.IsValid() {
		http.Error(w, "Method not found", http.StatusNotFound)
		return
	}

	var req APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	methodType := method.Type()
	in := make([]reflect.Value, methodType.NumIn())
	for i := 0; i < methodType.NumIn(); i++ {
		argType := methodType.In(i)
		if i < len(req.Args) {
			// Serialize and deserialize to handle proper type mapping (e.g., float64 to int, map to struct)
			b, _ := json.Marshal(req.Args[i])
			newVal := reflect.New(argType)
			if err := json.Unmarshal(b, newVal.Interface()); err == nil {
				in[i] = newVal.Elem()
			} else {
				in[i] = reflect.Zero(argType)
			}
		} else {
			in[i] = reflect.Zero(argType)
		}
	}

	// Safely call the method
	out := method.Call(in)

	var res APIResponse
	if len(out) > 0 {
		res.Result = out[0].Interface()
	}
	if len(out) > 1 && !out[1].IsNil() {
		errInterface := out[1].Interface()
		if err, ok := errInterface.(error); ok {
			res.Error = err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// StartHTTPServer starts the SaaS API server on the specified port.
func (a *App) StartHTTPServer(port int) {
	mux := http.NewServeMux()
	
	// REST API endpoint
	mux.HandleFunc("/api/", a.ServeHTTP)

	// WebSocket Hub endpoint for in-cluster Agents
	go a.hub.Run(a.ctx)
	mux.HandleFunc("/tunnel", a.hub.HandleTunnel)

	addr := fmt.Sprintf(":%d", port)
	a.logger.Info("Starting SaaS API Server", slog.String("addr", addr))
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			a.logger.Error("SaaS API Server failed", slog.String("error", err.Error()))
		}
	}()
}
