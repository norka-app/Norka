package biz

import (
	"fmt"

	"norka/internal/model"
)

// PlanProfileActivation decides how to bring a profile online.
//
// By default (stopOthers == false) it only starts the profile's tunnels.
// Tunnels outside the profile keep running. That avoids dropping a tunnel the
// user started by hand. When stopOthers is true, every running or connecting
// tunnel that is not in the profile is stopped first.
//
// A tunnel whose local port is already held is not started:
//   - held by another profile member (already up, or earlier in the profile
//     list and about to start) → conflict inside the profile;
//   - held by an outsider and stopOthers is false → conflict outside;
//   - held by an outsider and stopOthers is true → the outsider is in StopIDs,
//     so the member is started after that stop.
//
// Members that are already running, connecting, or reconnecting are left as
// they are (AlreadyIDs). Members in error or stopped are started.
func PlanProfileActivation(profile model.Profile, tunnels []model.Tunnel, stopOthers bool) model.ActivationPlan {
	byID := make(map[int]model.Tunnel, len(tunnels))
	for _, tunnel := range tunnels {
		byID[tunnel.ID] = tunnel
	}

	member := make(map[int]bool, len(profile.TunnelIDs))
	members := make([]model.Tunnel, 0, len(profile.TunnelIDs))
	for _, id := range profile.TunnelIDs {
		tunnel, ok := byID[id]
		if !ok || member[id] {
			continue
		}
		member[id] = true
		members = append(members, tunnel)
	}

	var plan model.ActivationPlan
	stopping := make(map[int]bool)
	if stopOthers {
		for _, tunnel := range tunnels {
			if member[tunnel.ID] || !holdsLocalPort(tunnel.Status) {
				continue
			}
			plan.StopIDs = append(plan.StopIDs, tunnel.ID)
			stopping[tunnel.ID] = true
		}
	}

	type holder struct {
		id     int
		name   string
		inside bool
	}
	held := make(map[string]holder)
	for _, tunnel := range tunnels {
		if tunnel.LocalPort <= 0 || !holdsLocalPort(tunnel.Status) || stopping[tunnel.ID] {
			continue
		}
		held[bindKey(tunnel)] = holder{id: tunnel.ID, name: tunnel.Name, inside: member[tunnel.ID]}
	}

	for _, tunnel := range members {
		if holdsLocalPort(tunnel.Status) {
			plan.AlreadyIDs = append(plan.AlreadyIDs, tunnel.ID)
			continue
		}
		if tunnel.LocalPort > 0 {
			if other, ok := held[bindKey(tunnel)]; ok {
				plan.Conflicts = append(plan.Conflicts, model.ProfileConflict{
					TunnelID:      tunnel.ID,
					TunnelName:    tunnel.Name,
					HolderID:      other.id,
					HolderName:    other.name,
					Port:          tunnel.LocalPort,
					InsideProfile: other.inside,
				})
				continue
			}
		}
		plan.StartIDs = append(plan.StartIDs, tunnel.ID)
		if tunnel.LocalPort > 0 {
			held[bindKey(tunnel)] = holder{id: tunnel.ID, name: tunnel.Name, inside: true}
		}
	}
	return plan
}

func bindKey(tunnel model.Tunnel) string {
	return fmt.Sprintf("%s:%d", normalizeBindHost(tunnel.LocalHost), tunnel.LocalPort)
}

// ApplyActivationPlan runs stops before starts. A failed stop or start is
// recorded and does not abort the rest. Conflicts are never started.
func ApplyActivationPlan(plan model.ActivationPlan, stopFn func(int) error, startFn func(int) error) model.ProfileActivationResult {
	result := model.ProfileActivationResult{
		Started:   []int{},
		Stopped:   []int{},
		AlreadyOn: append([]int{}, plan.AlreadyIDs...),
		Conflicts: append([]model.ProfileConflict{}, plan.Conflicts...),
		Errors:    []model.ProfileTunnelError{},
	}
	if result.AlreadyOn == nil {
		result.AlreadyOn = []int{}
	}
	if result.Conflicts == nil {
		result.Conflicts = []model.ProfileConflict{}
	}
	for _, id := range plan.StopIDs {
		if err := stopFn(id); err != nil {
			result.Errors = append(result.Errors, model.ProfileTunnelError{TunnelID: id, Action: "stop", Error: err.Error()})
			continue
		}
		result.Stopped = append(result.Stopped, id)
	}
	for _, id := range plan.StartIDs {
		if err := startFn(id); err != nil {
			result.Errors = append(result.Errors, model.ProfileTunnelError{TunnelID: id, Action: "start", Error: err.Error()})
			continue
		}
		result.Started = append(result.Started, id)
	}
	return result
}
