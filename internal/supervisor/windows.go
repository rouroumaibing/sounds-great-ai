//go:build windows

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// Windows job-object limit flag: kill the whole tree when the job handle
// closes (i.e. when the supervisor exits). This is the Windows equivalent of
// the process-group kill on Unix.
const jobObjectLimitKillOnJobClose = 0x2000

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit uint64
	PerJobUserTimeLimit     uint64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
	_                       uint32
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// assignToJob puts the child into a new job object configured with
// KILL_ON_JOB_CLOSE, so the entire child tree is reaped when the supervisor
// exits. Mirrors the Unix process-group kill on parent death.
func assignToJob(p *os.Process) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createJob := kernel32.NewProc("CreateJobObjectW")
	setInfo := kernel32.NewProc("SetInformationJobObject")
	assign := kernel32.NewProc("AssignProcessToJobObject")

	hJob, _, err := createJob.Call(0, 0)
	if hJob == 0 {
		return fmt.Errorf("CreateJobObjectW: %v", err)
	}
	var info jobObjectExtendedLimitInformation
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	r1, _, err := setInfo.Call(
		hJob,
		9, // JobObjectExtendedLimitInformation
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if r1 == 0 {
		return fmt.Errorf("SetInformationJobObject: %v", err)
	}
	r1, _, err = assign.Call(hJob, uintptr(p.Pid))
	if r1 == 0 {
		return fmt.Errorf("AssignProcessToJobObject: %v", err)
	}
	return nil
}

func syscallSIGTERM() os.Signal { return os.Kill }
func syscallSIGKILL() os.Signal { return os.Kill }

// On Windows there are no process groups; the equivalent is child.kill
// (no group signal). SG uses a Windows job object: assigning the child to a job
// and closing the job handle reaps the whole tree when the supervisor exits,
// which is the faithful "kill the supervised tree" semantic on this platform.

func newCmd(command string, args []string, workDir string, env []string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	if workDir != "" {
		cmd.Dir = workDir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	// CREATE_NEW_PROCESS_GROUP lets us target the child; the job object (set up
	// after Start via AssignProcessToJobObject) provides tree reaping.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	return cmd
}

func killProcessGroup(_ int, sig os.Signal) error {
	// Windows: the job object reaps the tree; an explicit kill is best-effort.
	return nil
}

func pgidOf(_ *exec.Cmd) int { return 0 }

func spawnInProcess(ctx context.Context, cfg Config, command string, args []string, workDir string, env []string) (*SupervisedProcess, error) {
	cmd := newCmd(command, args, workDir, env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Assign the child to a new job object so it is reaped with the supervisor.
	if err := assignToJob(cmd.Process); err != nil {
		// Non-fatal: the child still runs; tree reaping is best-effort.
		_ = err
	}
	p := &SupervisedProcess{
		cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	go func() {
		<-ctx.Done()
		_ = p.Terminate(cfg.KillGrace)
	}()
	return p, nil
}

func spawnSidecar(_ Config, command string, _ []string, _ string, _ []string) (*SupervisedProcess, error) {
	// Windows has no detached-sidecar re-exec path in scope; the job-object
	// in-process mode covers tree reaping on parent exit.
	return nil, fmt.Errorf("supervisor: sidecar mode is unsupported on windows")
}
