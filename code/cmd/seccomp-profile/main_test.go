package main

import "testing"

func evaluate(t *testing.T, program []instruction, architecture, syscall, arg0 uint32) uint32 {
	t.Helper()
	accumulator := uint32(0)
	for pc := 0; pc < len(program); pc++ {
		filter := program[pc]
		switch filter.code {
		case bpfLoadWordAbsolute:
			switch filter.k {
			case seccompDataArch:
				accumulator = architecture
			case seccompDataSyscall:
				accumulator = syscall
			case seccompDataArg0:
				accumulator = arg0
			default:
				t.Fatalf("unexpected load offset %d", filter.k)
			}
		case bpfJumpEqual:
			if accumulator == filter.k {
				pc += int(filter.jt)
			} else {
				pc += int(filter.jf)
			}
		case bpfReturn:
			return filter.k
		default:
			t.Fatalf("unexpected BPF opcode %#x", filter.code)
		}
	}
	t.Fatal("profile reached the end without returning")
	return 0
}

func TestProfiles(t *testing.T) {
	tests := []struct {
		name         string
		architecture string
		auditArch    uint32
		syscall      uint32
		arg0         uint32
		want         uint32
	}{
		{name: "amd64 vsock", architecture: "amd64", auditArch: auditArchAMD64, syscall: syscallSocketAMD64, arg0: addressFamilyVsock, want: seccompReturnErrnoEPERM},
		{name: "amd64 x32 vsock", architecture: "amd64", auditArch: auditArchAMD64, syscall: x32SyscallBit | syscallSocketAMD64, arg0: addressFamilyVsock, want: seccompReturnErrnoEPERM},
		{name: "amd64 inet", architecture: "amd64", auditArch: auditArchAMD64, syscall: syscallSocketAMD64, arg0: 2, want: seccompReturnAllow},
		{name: "amd64 io uring", architecture: "amd64", auditArch: auditArchAMD64, syscall: syscallIOUringSetup, want: seccompReturnErrnoEPERM},
		{name: "amd64 x32 io uring", architecture: "amd64", auditArch: auditArchAMD64, syscall: x32SyscallBit | syscallIOUringSetup, want: seccompReturnErrnoEPERM},
		{name: "amd64 read", architecture: "amd64", auditArch: auditArchAMD64, syscall: 0, want: seccompReturnAllow},
		{name: "amd64 wrong ABI", architecture: "amd64", auditArch: auditArchARM64, syscall: syscallSocketAMD64, arg0: addressFamilyVsock, want: seccompReturnKillProcess},
		{name: "arm64 vsock", architecture: "arm64", auditArch: auditArchARM64, syscall: syscallSocketARM64, arg0: addressFamilyVsock, want: seccompReturnErrnoEPERM},
		{name: "arm64 inet", architecture: "arm64", auditArch: auditArchARM64, syscall: syscallSocketARM64, arg0: 2, want: seccompReturnAllow},
		{name: "arm64 io uring", architecture: "arm64", auditArch: auditArchARM64, syscall: syscallIOUringSetup, want: seccompReturnErrnoEPERM},
		{name: "arm64 read", architecture: "arm64", auditArch: auditArchARM64, syscall: 63, want: seccompReturnAllow},
		{name: "arm64 wrong ABI", architecture: "arm64", auditArch: auditArchAMD64, syscall: syscallSocketARM64, arg0: addressFamilyVsock, want: seccompReturnKillProcess},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program, err := profile(test.architecture)
			if err != nil {
				t.Fatal(err)
			}
			if got := evaluate(t, program, test.auditArch, test.syscall, test.arg0); got != test.want {
				t.Fatalf("result = %#x, want %#x", got, test.want)
			}
			encoded, err := encode(program)
			if err != nil {
				t.Fatal(err)
			}
			if len(encoded) != len(program)*8 {
				t.Fatalf("encoded length = %d, want %d", len(encoded), len(program)*8)
			}
		})
	}
}

func TestUnsupportedArchitecture(t *testing.T) {
	if _, err := profile("386"); err == nil {
		t.Fatal("unsupported architecture was accepted")
	}
}
