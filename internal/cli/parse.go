package cli

import (
	"fmt"
	"strings"
)

// HiddenArg starts the GUI in the tray without showing the window.
// It is an internal switch used when `norka connect` launches the app
// and when login autostart is set to stay in the tray.
const HiddenArg = "--norka-hidden"

// Command is a parsed CLI invocation.
type Command struct {
	Op     string
	Target string
	JSON   bool
	Help   bool
}

// IsCommand reports whether args should be handled by the CLI instead of the GUI.
func IsCommand(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "connect", "disconnect", "toggle", "status", "list", "help", "--help", "-h", "--json":
			return true
		}
	}
	return false
}

// IsHiddenLaunch reports whether this process was started in the tray.
func IsHiddenLaunch(args []string) bool {
	for _, arg := range args {
		if arg == HiddenArg {
			return true
		}
	}
	return false
}

// ConsumeHidden removes the internal tray switch from argv.
func ConsumeHidden(args []string) ([]string, bool) {
	if len(args) == 0 {
		return args, false
	}
	out := make([]string, 0, len(args))
	out = append(out, args[0])
	hidden := false
	for _, arg := range args[1:] {
		if arg == HiddenArg {
			hidden = true
			continue
		}
		out = append(out, arg)
	}
	return out, hidden
}

// Parse reads CLI arguments without the program name.
func Parse(args []string) (Command, error) {
	var cmd Command
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--json":
			cmd.JSON = true
		case "--help", "-h", "help":
			cmd.Help = true
		default:
			if strings.HasPrefix(arg, "-") {
				return Command{}, fmt.Errorf("unknown flag %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if cmd.Help {
		return cmd, nil
	}
	if len(positional) == 0 {
		return Command{}, fmt.Errorf("missing command")
	}
	cmd.Op = positional[0]
	rest := positional[1:]
	switch cmd.Op {
	case "status", "list":
		if len(rest) != 0 {
			return Command{}, fmt.Errorf("%s does not take a tunnel name", cmd.Op)
		}
	case "connect", "disconnect", "toggle":
		if len(rest) != 1 || strings.TrimSpace(rest[0]) == "" {
			return Command{}, fmt.Errorf("%s needs one tunnel name or id", cmd.Op)
		}
		cmd.Target = rest[0]
	default:
		return Command{}, fmt.Errorf("unknown command %q", cmd.Op)
	}
	return cmd, nil
}
