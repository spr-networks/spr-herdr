package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	defaultColumns = 120
	defaultRows    = 36
)

type terminalStatus struct {
	Running    bool      `json:"running"`
	PID        int       `json:"pid,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	Generation uint64    `json:"generation"`
	BaseCursor uint64    `json:"baseCursor"`
	NextCursor uint64    `json:"nextCursor"`
	Columns    uint16    `json:"columns"`
	Rows       uint16    `json:"rows"`
	Version    string    `json:"version"`
	Transport  string    `json:"transport"`
}

type terminalSession struct {
	ctx     context.Context
	cancel  context.CancelFunc
	command string
	args    []string
	dir     string
	version string
	output  *outputRing

	mu         sync.Mutex
	pty        *os.File
	process    *os.Process
	running    bool
	startedAt  time.Time
	generation uint64
	columns    uint16
	rows       uint16
	done       chan struct{}
}

func newTerminalSession(command string, args []string, dir, version string, ringBytes int) *terminalSession {
	ctx, cancel := context.WithCancel(context.Background())
	return &terminalSession{
		ctx:     ctx,
		cancel:  cancel,
		command: command,
		args:    append([]string(nil), args...),
		dir:     dir,
		version: version,
		output:  newOutputRing(ringBytes),
		columns: defaultColumns,
		rows:    defaultRows,
		done:    make(chan struct{}),
	}
}

func (session *terminalSession) start() {
	go session.run()
}

func (session *terminalSession) close() {
	session.cancel()
	session.mu.Lock()
	process := session.process
	session.mu.Unlock()
	if process != nil {
		_ = process.Signal(syscall.SIGTERM)
	}
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		if process != nil {
			_ = process.Kill()
		}
		<-session.done
	}
}

func (session *terminalSession) run() {
	defer close(session.done)
	for {
		if session.ctx.Err() != nil {
			return
		}

		session.output.append([]byte("\r\n\x1b[2m[spr-herdr] attaching to Herdr...\x1b[0m\r\n"))
		err := session.runOnce()
		if session.ctx.Err() != nil {
			return
		}
		if err != nil {
			session.output.append([]byte(fmt.Sprintf("\r\n\x1b[31m[spr-herdr] terminal exited: %v\x1b[0m\r\n", err)))
		} else {
			session.output.append([]byte("\r\n\x1b[2m[spr-herdr] terminal detached; reconnecting...\x1b[0m\r\n"))
		}

		select {
		case <-session.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (session *terminalSession) runOnce() error {
	command := exec.CommandContext(session.ctx, session.command, session.args...)
	command.Dir = session.dir
	command.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	session.mu.Lock()
	size := &pty.Winsize{Cols: session.columns, Rows: session.rows}
	session.mu.Unlock()

	terminalPTY, err := pty.StartWithSize(command, size)
	if err != nil {
		return fmt.Errorf("start %s: %w", session.command, err)
	}

	session.mu.Lock()
	session.pty = terminalPTY
	session.process = command.Process
	session.running = true
	session.startedAt = time.Now().UTC()
	session.generation++
	session.mu.Unlock()

	buffer := make([]byte, 32*1024)
	for {
		count, readErr := terminalPTY.Read(buffer)
		if count > 0 {
			session.output.append(buffer[:count])
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, os.ErrClosed) {
				err = readErr
			}
			break
		}
	}

	waitErr := command.Wait()
	_ = terminalPTY.Close()

	session.mu.Lock()
	if session.pty == terminalPTY {
		session.pty = nil
		session.process = nil
		session.running = false
	}
	session.mu.Unlock()

	if session.ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return err
	}
	return waitErr
}

func (session *terminalSession) write(data []byte) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.running || session.pty == nil {
		return errors.New("terminal is not attached")
	}
	_, err := session.pty.Write(data)
	return err
}

func (session *terminalSession) resize(columns, rows uint16) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	session.columns = columns
	session.rows = rows
	if !session.running || session.pty == nil {
		return nil
	}
	return pty.Setsize(session.pty, &pty.Winsize{Cols: columns, Rows: rows})
}

func (session *terminalSession) redraw() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.running || session.pty == nil {
		return errors.New("terminal is not attached")
	}

	rows := session.rows
	if rows < ^uint16(0) {
		rows++
	} else {
		rows--
	}
	if err := pty.Setsize(session.pty, &pty.Winsize{Cols: session.columns, Rows: rows}); err != nil {
		return err
	}
	return pty.Setsize(session.pty, &pty.Winsize{Cols: session.columns, Rows: session.rows})
}

func (session *terminalSession) status() terminalStatus {
	session.mu.Lock()
	status := terminalStatus{
		Running:    session.running,
		StartedAt:  session.startedAt,
		Generation: session.generation,
		Columns:    session.columns,
		Rows:       session.rows,
		Version:    session.version,
		Transport:  "authenticated-http-long-poll",
	}
	if session.process != nil {
		status.PID = session.process.Pid
	}
	session.mu.Unlock()
	status.BaseCursor, status.NextCursor = session.output.bounds()
	return status
}
