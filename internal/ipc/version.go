package ipc

// VersionDecision is the result of comparing a client's dialect with this app.
// Reject is false for the legacy dialect (v omitted) and for a dialect this
// build still accepts. The handler must not run when Reject is true.
type VersionDecision struct {
	Reject   bool
	Response Response
}

// GateVersion compares client with MinClientVersion and ProtocolVersion.
// client 0 is the legacy dialect and is accepted.
func GateVersion(client int) VersionDecision {
	return gateVersion(client, MinClientVersion, ProtocolVersion)
}

func gateVersion(client, min, current int) VersionDecision {
	if client == 0 {
		return VersionDecision{}
	}
	if client < 0 || min < 1 || current < 1 {
		return VersionDecision{
			Reject: true,
			Response: Response{
				Code:     CodeBadRequest,
				ExitCode: ExitUsage,
				Message:  "bad request",
			},
		}
	}
	if client < min {
		return VersionDecision{
			Reject: true,
			Response: Response{
				Code:     CodeUpdateCLI,
				ExitCode: ExitVersion,
				Message:  MessageUpdateCLI,
			},
		}
	}
	if client > current {
		return VersionDecision{
			Reject: true,
			Response: Response{
				Code:     CodeUpdateApp,
				ExitCode: ExitVersion,
				Message:  MessageUpdateApp,
			},
		}
	}
	return VersionDecision{}
}
