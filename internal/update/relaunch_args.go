package update

import (
	"strconv"
	"strings"
	"time"
)

// AfterUpdateWaitFlag asks a newly started copy to wait until a process exits
// before taking the single-instance lock. It is an internal switch: startup
// removes it from os.Args so the UI and a later relaunch never see it.
const AfterUpdateWaitFlag = "--after-update-wait"

// afterUpdateWaitTimeout is how long the new process waits for the previous
// one. A missing or already exited PID does not wait.
const afterUpdateWaitTimeout = 30 * time.Second

// AfterUpdateWaitArgs is the argv suffix passed to the replacement executable.
func AfterUpdateWaitArgs(pid int) []string {
	if pid <= 0 {
		return nil
	}
	return []string{AfterUpdateWaitFlag, strconv.Itoa(pid)}
}

// ConsumeAfterUpdateWait removes the internal relaunch switch from args.
// pid is positive only when the switch carries a usable process id.
// The switch is removed even when the value is missing or invalid.
// args[0], the program name, is never treated as the switch.
func ConsumeAfterUpdateWait(args []string) (pid int, rest []string, found bool) {
	if len(args) < 2 {
		return 0, args, false
	}
	rest = make([]string, 0, len(args))
	rest = append(rest, args[0])
	for i := 1; i < len(args); i++ {
		arg := args[i]
		value, separate, ok := afterUpdateWaitValue(arg)
		if !ok {
			rest = append(rest, arg)
			continue
		}
		found = true
		if separate {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				value = args[i]
			} else {
				value = ""
			}
		}
		if pid == 0 {
			if parsed, valid := parseProcessID(value); valid {
				pid = parsed
			}
		}
	}
	if !found {
		return 0, args, false
	}
	return pid, rest, true
}

func afterUpdateWaitValue(arg string) (value string, separate bool, ok bool) {
	if arg == AfterUpdateWaitFlag {
		return "", true, true
	}
	prefix := AfterUpdateWaitFlag + "="
	if strings.HasPrefix(arg, prefix) {
		return strings.TrimPrefix(arg, prefix), false, true
	}
	return "", false, false
}

func parseProcessID(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "+") {
		return 0, false
	}
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || n == 0 {
		return 0, false
	}
	return int(n), true
}
