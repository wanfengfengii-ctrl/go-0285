package domain

// PourStep is one expected step of the continuous grouting sequence.
type PourStep struct {
	Type     EventType
	PortID   PortID
	SocketID SocketID
}

// PourSequence derives the full, ordered pour-step sequence from the locked
// plan. Each slurry path is an ordered list of inlet ports; every inlet is
// grouted in order and, for each socket, the outlet must first show stable
// flow and then be sealed before the switch to the next inlet.
func PourSequence(plan LockedPlan) []PourStep {
	var steps []PourStep
	for _, path := range plan.SlurryPaths {
		for i, inlet := range path {
			sock, ok := SocketOfPort(plan, inlet)
			if !ok {
				continue
			}
			outlet := outletPort(plan, sock)
			steps = append(steps,
				PourStep{Type: EventInletStart, PortID: inlet, SocketID: sock},
				PourStep{Type: EventOutletStable, PortID: outlet, SocketID: sock},
				PourStep{Type: EventOutletSeal, PortID: outlet, SocketID: sock},
			)
			if next := nextInlet(path, i); next != "" {
				steps = append(steps, PourStep{Type: EventPortSwitch, PortID: next, SocketID: sock})
			}
		}
	}
	return steps
}

// outletPort returns the outlet port of a socket.
func outletPort(plan LockedPlan, sock SocketID) PortID {
	for _, n := range plan.PortNodes {
		if n.SocketID == sock && n.Kind == PortOutlet {
			return n.ID
		}
	}
	return ""
}

// nextInlet returns the next inlet port in a slurry path, or "" when absent.
func nextInlet(path []PortID, i int) PortID {
	if i+1 < len(path) {
		return path[i+1]
	}
	return ""
}
