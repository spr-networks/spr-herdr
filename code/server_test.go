package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeTerminalSize(t *testing.T) {
	size, err := decodeTerminalSize(strings.NewReader(`{"columns":120,"rows":40}`))
	if err != nil {
		t.Fatal(err)
	}
	if size.Columns != 120 || size.Rows != 40 {
		t.Fatalf("size = %+v", size)
	}

	invalid := []string{
		`{"columns":1,"rows":40}`,
		`{"columns":120,"rows":501}`,
		`{"columns":120,"rows":40,"extra":true}`,
		`{"columns":120,"rows":40} {"columns":80,"rows":24}`,
	}
	for _, input := range invalid {
		if _, err := decodeTerminalSize(strings.NewReader(input)); err == nil {
			t.Fatalf("decodeTerminalSize(%q) unexpectedly succeeded", input)
		}
	}
}

func TestParseCursor(t *testing.T) {
	for _, test := range []struct {
		input string
		want  uint64
		ok    bool
	}{
		{"", 0, true},
		{"42", 42, true},
		{"-1", 0, false},
		{"not-a-number", 0, false},
	} {
		got, err := parseCursor(test.input)
		if (err == nil) != test.ok || got != test.want {
			t.Fatalf("parseCursor(%q) = %d, %v", test.input, got, err)
		}
	}
}

func TestTerminalOutputRequiresResetForExpiredCursor(t *testing.T) {
	session := &terminalSession{output: newOutputRing(5)}
	session.output.append([]byte("abcdefgh"))
	server := &terminalServer{session: session}

	request := httptest.NewRequest(http.MethodGet, "/terminal/output?cursor=0", nil)
	recorder := httptest.NewRecorder()
	server.terminalOutput(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
	if got := response.Header.Get("X-Terminal-Reset"); got != "required" {
		t.Fatalf("X-Terminal-Reset = %q, want required", got)
	}
	if got := response.Header.Get("X-Terminal-Base-Cursor"); got != "3" {
		t.Fatalf("X-Terminal-Base-Cursor = %q, want 3", got)
	}
	if got := response.Header.Get("X-Terminal-Next-Cursor"); got != "8" {
		t.Fatalf("X-Terminal-Next-Cursor = %q, want 8", got)
	}
}
