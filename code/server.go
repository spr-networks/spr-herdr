package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	maxInputBytes  = 64 * 1024
	maxOutputBytes = 128 * 1024
	maxColumns     = 1000
	maxRows        = 500
	pollTimeout    = 25 * time.Second
)

//go:embed ui/*
var embeddedUI embed.FS

type terminalServer struct {
	session *terminalSession
	static  http.Handler
}

func newTerminalServer(session *terminalSession) (*terminalServer, error) {
	staticFiles, err := fs.Sub(embeddedUI, "ui")
	if err != nil {
		return nil, fmt.Errorf("open embedded UI: %w", err)
	}
	return &terminalServer{
		session: session,
		static:  http.FileServer(http.FS(staticFiles)),
	}, nil
}

func (server *terminalServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.serveIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", server.securityHeaders(server.static)))
	mux.HandleFunc("/healthz", server.health)
	mux.HandleFunc("/terminal/status", server.terminalStatus)
	mux.HandleFunc("/terminal/output", server.terminalOutput)
	mux.HandleFunc("/terminal/input", server.terminalInput)
	mux.HandleFunc("/terminal/resize", server.terminalResize)
	mux.HandleFunc("/terminal/redraw", server.terminalRedraw)
	return server.securityHeaders(mux)
}

func (server *terminalServer) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
		next.ServeHTTP(writer, request)
	})
}

func requireMethod(writer http.ResponseWriter, request *http.Request, method string) bool {
	if request.Method == method {
		return true
	}
	writer.Header().Set("Allow", method)
	http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (server *terminalServer) serveIndex(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	if !requireMethod(writer, request, http.MethodGet) {
		return
	}
	index, err := embeddedUI.ReadFile("ui/index.html")
	if err != nil {
		http.Error(writer, "terminal UI is unavailable", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = writer.Write(index)
}

func (server *terminalServer) health(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodGet) {
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok":      true,
		"running": server.session.status().Running,
	})
}

func (server *terminalServer) terminalStatus(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodGet) {
		return
	}
	writeJSON(writer, http.StatusOK, server.session.status())
}

func parseCursor(raw string) (uint64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return strconv.ParseUint(raw, 10, 64)
}

func setCursorHeaders(writer http.ResponseWriter, snapshot outputSnapshot) {
	writer.Header().Set("X-Terminal-Base-Cursor", strconv.FormatUint(snapshot.base, 10))
	writer.Header().Set("X-Terminal-Next-Cursor", strconv.FormatUint(snapshot.next, 10))
}

func (server *terminalServer) terminalOutput(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodGet) {
		return
	}
	cursor, err := parseCursor(request.URL.Query().Get("cursor"))
	if err != nil {
		http.Error(writer, "cursor must be an unsigned integer", http.StatusBadRequest)
		return
	}

	timer := time.NewTimer(pollTimeout)
	defer timer.Stop()
	for {
		snapshot := server.session.output.read(cursor, maxOutputBytes)
		setCursorHeaders(writer, snapshot)
		if snapshot.stale {
			writer.Header().Set("X-Terminal-Reset", "required")
			writeJSON(writer, http.StatusConflict, map[string]any{
				"error":      "terminal replay cursor expired",
				"reset":      "required",
				"baseCursor": snapshot.base,
				"nextCursor": snapshot.next,
			})
			return
		}
		if len(snapshot.data) > 0 {
			writer.Header().Set("Content-Type", "application/octet-stream")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(snapshot.data)
			return
		}

		select {
		case <-request.Context().Done():
			return
		case <-snapshot.changed:
			continue
		case <-timer.C:
			writer.WriteHeader(http.StatusNoContent)
			return
		}
	}
}

func (server *terminalServer) terminalInput(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodPost) {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, maxInputBytes))
	if err != nil {
		http.Error(writer, "terminal input is too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(data) == 0 {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	if err := server.session.write(data); err != nil {
		http.Error(writer, err.Error(), http.StatusConflict)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

type terminalSize struct {
	Columns uint16 `json:"columns"`
	Rows    uint16 `json:"rows"`
}

func decodeTerminalSize(reader io.Reader) (terminalSize, error) {
	var size terminalSize
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&size); err != nil {
		return terminalSize{}, err
	}
	if size.Columns < 2 || size.Columns > maxColumns || size.Rows < 2 || size.Rows > maxRows {
		return terminalSize{}, errors.New("terminal size is outside the supported range")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return terminalSize{}, errors.New("terminal size must contain one JSON object")
	}
	return size, nil
}

func (server *terminalServer) terminalResize(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodPost) {
		return
	}
	size, err := decodeTerminalSize(http.MaxBytesReader(writer, request.Body, 4*1024))
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := server.session.resize(size.Columns, size.Rows); err != nil {
		http.Error(writer, "could not resize terminal", http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (server *terminalServer) terminalRedraw(writer http.ResponseWriter, request *http.Request) {
	if !requireMethod(writer, request, http.MethodPost) {
		return
	}
	if err := server.session.redraw(); err != nil {
		http.Error(writer, err.Error(), http.StatusConflict)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
