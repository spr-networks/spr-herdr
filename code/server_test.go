package main

import (
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
