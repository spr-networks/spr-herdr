package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
)

const (
	bpfLoadWordAbsolute = 0x20
	bpfJumpEqual        = 0x15
	bpfReturn           = 0x06

	seccompDataSyscall = 0
	seccompDataArch    = 4
	seccompDataArg0    = 16

	auditArchAMD64 = 0xc000003e
	auditArchARM64 = 0xc00000b7

	syscallSocketAMD64  = 41
	syscallSocketARM64  = 198
	syscallIOUringSetup = 425
	x32SyscallBit       = 0x40000000

	addressFamilyVsock = 40

	seccompReturnKillProcess = 0x80000000
	seccompReturnErrnoEPERM  = 0x00050001
	seccompReturnAllow       = 0x7fff0000
)

type instruction struct {
	code uint16
	jt   uint8
	jf   uint8
	k    uint32
}

func load(offset uint32) instruction {
	return instruction{code: bpfLoadWordAbsolute, k: offset}
}

func jumpEqual(value uint32, jumpTrue, jumpFalse uint8) instruction {
	return instruction{code: bpfJumpEqual, jt: jumpTrue, jf: jumpFalse, k: value}
}

func result(value uint32) instruction {
	return instruction{code: bpfReturn, k: value}
}

func profile(architecture string) ([]instruction, error) {
	var auditArchitecture uint32
	var socketSyscall uint32
	var alternateSocketSyscall uint32
	var alternateIOUringSyscall uint32

	switch architecture {
	case "amd64":
		auditArchitecture = auditArchAMD64
		socketSyscall = syscallSocketAMD64
		alternateSocketSyscall = x32SyscallBit | syscallSocketAMD64
		alternateIOUringSyscall = x32SyscallBit | syscallIOUringSetup
	case "arm64":
		auditArchitecture = auditArchARM64
		socketSyscall = syscallSocketARM64
		alternateSocketSyscall = syscallSocketARM64
		alternateIOUringSyscall = syscallIOUringSetup
	default:
		return nil, fmt.Errorf("unsupported architecture %q", architecture)
	}

	return []instruction{
		load(seccompDataArch),
		jumpEqual(auditArchitecture, 1, 0),
		result(seccompReturnKillProcess),
		load(seccompDataSyscall),
		jumpEqual(socketSyscall, 4, 0),
		jumpEqual(alternateSocketSyscall, 3, 0),
		jumpEqual(syscallIOUringSetup, 5, 0),
		jumpEqual(alternateIOUringSyscall, 4, 0),
		result(seccompReturnAllow),
		load(seccompDataArg0),
		jumpEqual(addressFamilyVsock, 1, 0),
		result(seccompReturnAllow),
		result(seccompReturnErrnoEPERM),
	}, nil
}

func encode(program []instruction) ([]byte, error) {
	if len(program) == 0 {
		return nil, errors.New("seccomp profile is empty")
	}

	encoded := make([]byte, len(program)*8)
	for index, filter := range program {
		offset := index * 8
		binary.LittleEndian.PutUint16(encoded[offset:], filter.code)
		encoded[offset+2] = filter.jt
		encoded[offset+3] = filter.jf
		binary.LittleEndian.PutUint32(encoded[offset+4:], filter.k)
	}
	return encoded, nil
}

func writeProfile(architecture, outputPath string) error {
	program, err := profile(architecture)
	if err != nil {
		return err
	}
	encoded, err := encode(program)
	if err != nil {
		return err
	}
	if outputPath == "" {
		return errors.New("output path is required")
	}
	return os.WriteFile(outputPath, encoded, 0o644)
}

func main() {
	architecture := flag.String("arch", "", "target architecture")
	outputPath := flag.String("output", "", "output BPF profile")
	flag.Parse()

	if err := writeProfile(*architecture, *outputPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
