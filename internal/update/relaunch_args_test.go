package update

import (
	"reflect"
	"testing"
	"time"
)

func TestAfterUpdateWaitArgs(t *testing.T) {
	got := AfterUpdateWaitArgs(8125)
	want := []string{AfterUpdateWaitFlag, "8125"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v", got)
	}
	if AfterUpdateWaitArgs(0) != nil || AfterUpdateWaitArgs(-1) != nil {
		t.Fatal("non-positive pid should not produce a switch")
	}
}

func TestAfterUpdateWaitTimeout(t *testing.T) {
	if afterUpdateWaitTimeout != 30*time.Second {
		t.Fatalf("timeout = %s", afterUpdateWaitTimeout)
	}
}

func TestConsumeAfterUpdateWait(t *testing.T) {
	original := []string{"norka.exe", "--norka-focus=3"}
	pid, rest, found := ConsumeAfterUpdateWait(original)
	if found || pid != 0 || !reflect.DeepEqual(rest, original) {
		t.Fatalf("unchanged args: pid=%d rest=%#v found=%v", pid, rest, found)
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{
		`C:\Program Files\norka.exe`,
		"--after-update-wait",
		"8125",
		"--norka-focus=3",
	})
	if !found || pid != 8125 {
		t.Fatalf("pid=%d found=%v", pid, found)
	}
	if !reflect.DeepEqual(rest, []string{`C:\Program Files\norka.exe`, "--norka-focus=3"}) {
		t.Fatalf("rest = %#v", rest)
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{"norka", "--after-update-wait=44", "keep"})
	if !found || pid != 44 || !reflect.DeepEqual(rest, []string{"norka", "keep"}) {
		t.Fatalf("equals form: pid=%d rest=%#v found=%v", pid, rest, found)
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{"norka", "--after-update-wait"})
	if !found || pid != 0 || !reflect.DeepEqual(rest, []string{"norka"}) {
		t.Fatalf("missing value: pid=%d rest=%#v found=%v", pid, rest, found)
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{"norka", "--after-update-wait", "--norka-focus=1"})
	if !found || pid != 0 || !reflect.DeepEqual(rest, []string{"norka", "--norka-focus=1"}) {
		t.Fatalf("following flag kept: pid=%d rest=%#v found=%v", pid, rest, found)
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{"norka", "--after-update-wait", "-1"})
	if !found || pid != 0 || !reflect.DeepEqual(rest, []string{"norka", "-1"}) {
		t.Fatalf("negative token kept: pid=%d rest=%#v found=%v", pid, rest, found)
	}

	cases := []string{"0", "abc", "999999999999", "+12", ""}
	for _, value := range cases {
		args := []string{"norka", "--after-update-wait", value}
		if value == "" {
			args = []string{"norka", "--after-update-wait="}
		}
		pid, rest, found = ConsumeAfterUpdateWait(args)
		if !found || pid != 0 {
			t.Fatalf("value %q: pid=%d found=%v", value, pid, found)
		}
		if len(rest) != 1 || rest[0] != "norka" {
			t.Fatalf("value %q leaked into %#v", value, rest)
		}
	}

	pid, rest, found = ConsumeAfterUpdateWait([]string{
		"norka",
		"--after-update-wait",
		"nope",
		"--after-update-wait",
		"15",
		"--after-update-waiting",
	})
	if !found || pid != 15 || !reflect.DeepEqual(rest, []string{"norka", "--after-update-waiting"}) {
		t.Fatalf("first valid pid: pid=%d rest=%#v found=%v", pid, rest, found)
	}
}

func TestWaitForPIDExitNoopOnUnusableID(t *testing.T) {
	start := time.Now()
	WaitForPIDExit(0)
	WaitForPIDExit(-4)
	if time.Since(start) > time.Second {
		t.Fatal("unusable pid should return immediately")
	}
}
