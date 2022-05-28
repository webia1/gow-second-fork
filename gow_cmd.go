package main

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/mitranim/gg"
)

type Cmd struct {
	sync.RWMutex
	Buf   [1]byte
	Err   gg.Chan[error]
	Cmd   *exec.Cmd
	Stdin io.WriteCloser
}

func (self *Cmd) Init() { self.Err.InitCap(1) }

func (self *Cmd) Deinit() {
	defer gg.Lock(self).Unlock()
	self.DeinitUnsync()
}

func (self *Cmd) DeinitUnsync() {
	self.BroadcastUnsync(syscall.SIGTERM)
	self.Cmd = nil
	self.Stdin = nil
}

func (self *Cmd) Restart(cmd *exec.Cmd) {
	defer gg.Lock(self).Unlock()

	self.DeinitUnsync()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf(`unable to initialize subcommand stdin: %v`, err)
		return
	}

	// Starting the subprocess populates its `.Process`,
	// which allows us to kill the subprocess group on demand.
	err = cmd.Start()
	if err != nil {
		log.Printf(`unable to start subcommand: %v`, err)
		return
	}

	self.Cmd = cmd
	self.Stdin = stdin
	go cmdWait(cmd, self.Err)
}

func (self *Cmd) Has() bool {
	return gg.LockDeref(self.RLocker(), &self.Cmd) != nil
}

func (self *Cmd) Broadcast(sig syscall.Signal) {
	defer gg.Lock(self).Unlock()
	self.BroadcastUnsync(sig)
}

/**
Sends the signal to the subprocess group, denoted by the negative sign on the
PID. Requires `syscall.SysProcAttr{Setpgid: true}`.
*/
func (self *Cmd) BroadcastUnsync(sig syscall.Signal) {
	proc := self.ProcUnsync()
	if proc != nil {
		gg.Nop1(syscall.Kill(-proc.Pid, sig))
	}
}

func (self *Cmd) WriteChar(char byte) {
	stdin := gg.LockDeref(self.RLocker(), &self.Stdin)
	buf := &self.Buf

	if stdin != nil {
		buf[0] = char
		gg.Write(stdin, buf[:])
	}
}

func (self *Cmd) ProcUnsync() *os.Process {
	cmd := self.Cmd
	if cmd != nil {
		return cmd.Process
	}
	return nil
}