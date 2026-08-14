package main

import "testing"

func TestConfigValidation(t *testing.T) {
	valid := config{
		socketPath: "/run/spr-herdr/ui.sock",
		command:    "/usr/local/bin/herdr",
		workdir:    "/home/herdr/workspace",
		ringBytes:  defaultRingBytes,
	}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}

	tests := []config{
		{socketPath: "relative.sock", command: valid.command, workdir: valid.workdir, ringBytes: valid.ringBytes},
		{socketPath: valid.socketPath, command: "herdr", workdir: valid.workdir, ringBytes: valid.ringBytes},
		{socketPath: valid.socketPath, command: valid.command, workdir: "workspace", ringBytes: valid.ringBytes},
		{socketPath: valid.socketPath, command: valid.command, workdir: valid.workdir, ringBytes: 1},
	}
	for _, cfg := range tests {
		if err := cfg.validate(); err == nil {
			t.Fatalf("invalid config unexpectedly passed: %+v", cfg)
		}
	}
}
