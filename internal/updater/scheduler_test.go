package updater

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

type schedulerCall struct {
	name string
	args []string
}

type stubRunner struct {
	calls []schedulerCall
	reply func(name string, args []string) ([]byte, error)
}

func (s *stubRunner) Run(name string, args ...string) ([]byte, error) {
	s.calls = append(s.calls, schedulerCall{name: name, args: append([]string(nil), args...)})
	if s.reply == nil {
		return nil, nil
	}
	return s.reply(name, args)
}

func (s *stubRunner) callsWithFlag(flag string) []schedulerCall {
	var matched []schedulerCall
	for _, call := range s.calls {
		if slices.Contains(call.args, flag) {
			matched = append(matched, call)
		}
	}
	return matched
}

// windowsQuery fakes schtasks: an empty definition mimics an unregistered name.
func windowsQuery(definition string) *stubRunner {
	return &stubRunner{reply: func(_ string, args []string) ([]byte, error) {
		if len(args) == 0 || args[0] != "/Query" {
			return nil, nil
		}
		if definition == "" {
			return []byte("ERROR: The system cannot find the file specified."), errors.New("exit status 1")
		}
		return []byte(definition), nil
	}}
}

// crontabStub fakes the user crontab: an empty listed value means no crontab exists.
type crontabStub struct {
	listed  string
	written string
}

func (c *crontabStub) reply() func(string, []string) ([]byte, error) {
	return func(_ string, args []string) ([]byte, error) {
		if args[0] == "-l" {
			if c.listed == "" {
				return []byte("no crontab for user\n"), errors.New("exit status 1")
			}
			return []byte(c.listed), nil
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return nil, err
		}
		c.written = string(data)
		return nil, nil
	}
}

func withSchedulerGOOS(t *testing.T, goos string) {
	t.Helper()
	previous := schedulerGOOS
	schedulerGOOS = goos
	t.Cleanup(func() { schedulerGOOS = previous })
}

func argAfter(args []string, flag string) string {
	if i := slices.Index(args, flag); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return ""
}

func TestSchedulerWindowsEnableRegistersCheckOnlyTask(t *testing.T) {
	withSchedulerGOOS(t, "windows")
	execPath := `C:\Program Files\cortex-ia\cortex-ia.exe`
	runner := windowsQuery("")
	if err := EnableScheduledCheck(execPath, runner); err != nil {
		t.Fatalf("EnableScheduledCheck: %v", err)
	}

	creates := runner.callsWithFlag("/Create")
	if len(creates) != 1 {
		t.Fatalf("expected exactly one schtasks /Create, got %d: %+v", len(creates), runner.calls)
	}
	create := creates[0]
	if create.name != "schtasks" || argAfter(create.args, "/TN") != ScheduledTaskName {
		t.Fatalf("unexpected create invocation: %s %+v", create.name, create.args)
	}
	if got := argAfter(create.args, "/SC"); got != "DAILY" {
		t.Errorf("schedule = %q, want DAILY", got)
	}
	if got, want := argAfter(create.args, "/TR"), `"`+execPath+`" `+ScheduledCheckArgs; got != want {
		t.Errorf("task action = %q, want %q", got, want)
	}
	for _, elevated := range []string{"/RL", "/RU", "/RP"} {
		if argAfter(create.args, elevated) != "" {
			t.Errorf("registration must stay user-level, found %s", elevated)
		}
	}
}

func TestSchedulerWindowsStatusAndIdempotentDisable(t *testing.T) {
	withSchedulerGOOS(t, "windows")
	execPath := `C:\Program Files\cortex-ia\cortex-ia.exe`
	definition := `"TaskName","Task To Run"` + "\r\n" +
		`"CortexIA Update Check","\"` + execPath + `\" ` + ScheduledCheckArgs + `"`

	status, err := ScheduledCheckStatus(windowsQuery(definition))
	if err != nil {
		t.Fatalf("ScheduledCheckStatus: %v", err)
	}
	if !status.Registered || status.Executable != execPath || status.Frequency != ScheduledCheckFrequency {
		t.Fatalf("unexpected status: %+v", status)
	}
	absent := windowsQuery("")
	if err := DisableScheduledCheck(absent); err != nil {
		t.Fatalf("DisableScheduledCheck: %v", err)
	}
	if deletes := absent.callsWithFlag("/Delete"); len(deletes) != 0 {
		t.Fatalf("expected no delete invocation, got %+v", deletes)
	}
}

func TestSchedulerWindowsRefusesForeignTaskUnderManagedName(t *testing.T) {
	withSchedulerGOOS(t, "windows")
	foreign := windowsQuery(`"CortexIA Update Check","C:\other\tool.exe --sync"`)

	if err := EnableScheduledCheck(`C:\cortex-ia.exe`, foreign); !errors.Is(err, ErrForeignScheduledTask) {
		t.Fatalf("enable error = %v, want ErrForeignScheduledTask", err)
	}
	if creates := foreign.callsWithFlag("/Create"); len(creates) != 0 {
		t.Fatalf("a foreign task must never be overwritten, got %+v", creates)
	}
	if err := DisableScheduledCheck(foreign); !errors.Is(err, ErrForeignScheduledTask) {
		t.Fatalf("disable error = %v, want ErrForeignScheduledTask", err)
	}
}

func TestSchedulerMissingCommandDegradesClearly(t *testing.T) {
	withSchedulerGOOS(t, "windows")
	missing := &stubRunner{reply: func(name string, _ []string) ([]byte, error) {
		return nil, &exec.Error{Name: name, Err: exec.ErrNotFound}
	}}

	checks := map[string]func() error{
		"enable":  func() error { return EnableScheduledCheck(`C:\cortex-ia.exe`, missing) },
		"disable": func() error { return DisableScheduledCheck(missing) },
		"status": func() error {
			_, err := ScheduledCheckStatus(missing)
			return err
		},
	}
	for name, check := range checks {
		if err := check(); !errors.Is(err, ErrSchedulerUnavailable) {
			t.Fatalf("%s error = %v, want ErrSchedulerUnavailable", name, err)
		}
	}
}

func TestSchedulerPOSIXEnableUsesQuotedCheckOnlyPayload(t *testing.T) {
	withSchedulerGOOS(t, "linux")
	execPath := "/home/user/bin dir/cortex-ia"
	stub := &crontabStub{}

	if err := EnableScheduledCheck(execPath, &stubRunner{reply: stub.reply()}); err != nil {
		t.Fatalf("EnableScheduledCheck: %v", err)
	}
	line := strings.TrimSpace(stub.written)
	for _, want := range []string{`"/home/user/bin dir/cortex-ia"`, ScheduledCheckArgs, scheduledCronMarker} {
		if !strings.Contains(line, want) {
			t.Errorf("cron line %q is missing %q", line, want)
		}
	}
	for _, forbidden := range []string{"apply", "sudo"} {
		if strings.Contains(line, forbidden) {
			t.Errorf("cron line %q must stay check-only and user-level", line)
		}
	}
	status, err := ScheduledCheckStatus(&stubRunner{reply: func(string, []string) ([]byte, error) {
		return []byte(stub.written), nil
	}})
	if err != nil {
		t.Fatalf("ScheduledCheckStatus: %v", err)
	}
	if !status.Registered || status.Executable != execPath {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestSchedulerPOSIXDisableIsSurgical(t *testing.T) {
	withSchedulerGOOS(t, "linux")
	stub := &crontabStub{listed: "*/5 * * * * /usr/bin/backup\n" +
		`0 12 * * * "/home/u/cortex-ia" update --check --scheduled # cortex-ia-managed scheduled update check` + "\n"}

	if err := DisableScheduledCheck(&stubRunner{reply: stub.reply()}); err != nil {
		t.Fatalf("DisableScheduledCheck: %v", err)
	}
	if !strings.Contains(stub.written, "/usr/bin/backup") {
		t.Fatalf("unrelated crontab entries must survive disable: %q", stub.written)
	}
	if strings.Contains(stub.written, ScheduledCheckArgs) {
		t.Fatalf("managed entry was not removed: %q", stub.written)
	}

	empty := &crontabStub{}
	if err := DisableScheduledCheck(&stubRunner{reply: empty.reply()}); err != nil {
		t.Fatalf("DisableScheduledCheck without crontab: %v", err)
	}
	if empty.written != "" {
		t.Fatalf("no crontab write expected, got %q", empty.written)
	}
}

func TestSchedulerPlatformAndPathGuards(t *testing.T) {
	runner := &stubRunner{}
	withSchedulerGOOS(t, "plan9")

	if err := EnableScheduledCheck("/bin/cortex-ia", runner); !errors.Is(err, ErrUnsupportedSchedulerPlatform) {
		t.Fatalf("enable error = %v, want ErrUnsupportedSchedulerPlatform", err)
	}
	if _, err := ScheduledCheckStatus(runner); !errors.Is(err, ErrUnsupportedSchedulerPlatform) {
		t.Fatalf("status error = %v, want ErrUnsupportedSchedulerPlatform", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("no scheduler command may run on an unsupported platform: %+v", runner.calls)
	}

	withSchedulerGOOS(t, "windows")
	if err := EnableScheduledCheck("   ", runner); !errors.Is(err, ErrMissingScheduledExecutable) {
		t.Fatalf("blank executable error = %v, want ErrMissingScheduledExecutable", err)
	}
}
