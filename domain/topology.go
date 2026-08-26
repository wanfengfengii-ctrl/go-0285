package domain

import "sort"

// ValidationIssue is a single rejection reason discovered while validating a
// lock request. Issues are collected in bulk, then sorted by their stable
// domain key before the whole lock is rejected, so the reported reasons are
// deterministic across runs and restarts.
type ValidationIssue struct {
	Code Code
	Key  string
	Msg  string
}

// ValidateLockPlan checks a candidate LockedPlan against the catalog and the
// task's building/level/wall identity. It returns every mismatch at once,
// ordered by stable domain key. A nil slice means the plan is valid.
func ValidateLockPlan(building, level, wallPanel string, plan LockedPlan, catalog CatalogSnapshot) []ValidationIssue {
	var issues []ValidationIssue

	if plan.WallPosition.Building != building || plan.WallPosition.Level != level || plan.WallPosition.WallID != wallPanel {
		issues = append(issues, ValidationIssue{
			Code: CodeWallPanelMismatch,
			Key:  "wall",
			Msg:  "wall position does not match task identity",
		})
	}

	// Every connection must use a catalog-compatible rebar/sleeve pair.
	compat := catalog.compatSet()
	for i, c := range plan.Connections {
		if !compat[c.RebarSpec+"|"+c.SleeveSpec] {
			issues = append(issues, ValidationIssue{
				Code: CodeSocketIncompatible,
				Key:  "connection/" + string(c.SocketID),
				Msg:  "rebar " + c.RebarSpec + " incompatible with sleeve " + c.SleeveSpec,
			})
		}
		_ = i
	}

	// Each sleeve must expose exactly one inlet and one outlet port.
	inletCount := map[SocketID]int{}
	outletCount := map[SocketID]int{}
	seenPort := map[PortID]bool{}
	for _, n := range plan.PortNodes {
		if seenPort[n.ID] {
			issues = append(issues, ValidationIssue{
				Code: CodeInvalidTopology,
				Key:  "port/" + string(n.ID),
				Msg:  "duplicate port " + string(n.ID),
			})
		}
		seenPort[n.ID] = true
		if n.Kind == PortInlet {
			inletCount[n.SocketID]++
		} else if n.Kind == PortOutlet {
			outletCount[n.SocketID]++
		}
	}
	socketSeen := map[SocketID]bool{}
	for _, c := range plan.Connections {
		socketSeen[c.SocketID] = true
	}
	for sid := range socketSeen {
		if inletCount[sid] != 1 || outletCount[sid] != 1 {
			issues = append(issues, ValidationIssue{
				Code: CodeInvalidTopology,
				Key:  "sleeve/" + string(sid),
				Msg:  "sleeve " + string(sid) + " must have exactly one inlet and one outlet",
			})
		}
	}

	// Topology must be connected and acyclic.
	if err := checkConnectivity(plan); err != "" {
		issues = append(issues, ValidationIssue{
			Code: CodeInvalidTopology,
			Key:  "topology/connectivity",
			Msg:  err,
		})
	}

	// Theoretical volume and loss ceiling must sit inside catalog bounds.
	if plan.TheoreticalVolumeML <= 0 {
		issues = append(issues, ValidationIssue{
			Code: CodeFixedPointOverflow,
			Key:  "volume/nonpositive",
			Msg:  "theoretical volume must be positive",
		})
	} else if plan.TheoreticalVolumeML < catalog.LossBounds.MinVolumeML || plan.TheoreticalVolumeML > catalog.LossBounds.MaxVolumeML {
		issues = append(issues, ValidationIssue{
			Code: CodeMaterialImbalance,
			Key:  "volume/bounds",
			Msg:  "theoretical volume outside catalog bounds",
		})
	}
	if plan.LossCeilingPPM < 0 || plan.LossCeilingPPM > catalog.LossBounds.MaxLossRatioPPM {
		issues = append(issues, ValidationIssue{
			Code: CodeMaterialImbalance,
			Key:  "loss/bounds",
			Msg:  "loss ceiling outside catalog bounds",
		})
	}

	// Material and water batches must be certified by the catalog.
	if !catalog.certifiedBatch(plan.MaterialBatch) {
		issues = append(issues, ValidationIssue{
			Code: CodeInvalidTopology,
			Key:  "batch/material",
			Msg:  "material batch " + plan.MaterialBatch + " is not certified",
		})
	}
	if !catalog.certifiedBatch(plan.WaterBatch) {
		issues = append(issues, ValidationIssue{
			Code: CodeInvalidTopology,
			Key:  "batch/water",
			Msg:  "water batch " + plan.WaterBatch + " is not certified",
		})
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].Key < issues[j].Key })
	return issues
}

func (c CatalogSnapshot) compatSet() map[string]bool {
	m := make(map[string]bool, len(c.Compat))
	for _, sc := range c.Compat {
		m[sc.RebarSpec+"|"+sc.SleeveSpec] = true
	}
	return m
}

func (c CatalogSnapshot) certifiedBatch(batch string) bool {
	for _, mc := range c.MaterialCert {
		if mc.BatchID == batch {
			return true
		}
	}
	return false
}

// checkConnectivity verifies the port graph is connected and acyclic. It
// returns a human-readable reason on failure and "" when valid.
func checkConnectivity(plan LockedPlan) string {
	if len(plan.PortNodes) == 0 {
		return "topology has no ports"
	}
	adj := map[PortID][]PortID{}
	inDegree := map[PortID]int{}
	for _, n := range plan.PortNodes {
		inDegree[n.ID] = 0
	}
	for _, e := range plan.PortEdges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}
	// Cycle detection via Kahn's algorithm: a DAG has a topological ordering.
	queue := []PortID{}
	for id, d := range inDegree {
		if d == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, to := range adj[id] {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	if visited != len(plan.PortNodes) {
		return "topology contains a cycle or is disconnected"
	}
	// Connectivity: every port must be reachable from some inlet root.
	reachable := reachableFrom(plan, adj)
	if len(reachable) != len(plan.PortNodes) {
		return "topology is not fully connected"
	}
	return ""
}

func reachableFrom(plan LockedPlan, adj map[PortID][]PortID) map[PortID]bool {
	seen := map[PortID]bool{}
	var stack []PortID
	for _, n := range plan.PortNodes {
		if n.Kind == PortInlet {
			stack = append(stack, n.ID)
		}
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, adj[id]...)
	}
	return seen
}

// SocketOfPort returns the socket a port belongs to.
func SocketOfPort(plan LockedPlan, port PortID) (SocketID, bool) {
	for _, n := range plan.PortNodes {
		if n.ID == port {
			return n.SocketID, true
		}
	}
	return "", false
}

// ComputeRecheckSet deterministically expands a set of defect sockets into the
// ordered, de-duplicated recheck set: each defect, plus the sockets of every
// successor port on the same slurry path, plus each directly adjacent socket.
// The result is sorted by socket id for a stable, reproducible order.
func ComputeRecheckSet(plan LockedPlan, defects []SocketID) []SocketID {
	included := map[SocketID]bool{}

	// Direct adjacency derived from port edges.
	adjacent := map[SocketID]map[SocketID]bool{}
	for _, e := range plan.PortEdges {
		from, ok1 := SocketOfPort(plan, e.From)
		to, ok2 := SocketOfPort(plan, e.To)
		if ok1 && ok2 {
			if adjacent[from] == nil {
				adjacent[from] = map[SocketID]bool{}
			}
			if adjacent[to] == nil {
				adjacent[to] = map[SocketID]bool{}
			}
			adjacent[from][to] = true
			adjacent[to][from] = true
		}
	}

	// Index slurry paths by socket for successor expansion.
	socketPaths := map[SocketID][]int{} // path index -> list of path indices containing the socket
	for pi, path := range plan.SlurryPaths {
		for _, p := range path {
			if sid, ok := SocketOfPort(plan, p); ok {
				socketPaths[sid] = append(socketPaths[sid], pi)
			}
		}
	}

	for _, d := range defects {
		included[d] = true
		for a := range adjacent[d] {
			included[a] = true
		}
		for _, pi := range socketPaths[d] {
			path := plan.SlurryPaths[pi]
			// Include every socket after the first occurrence of d in the path.
			after := false
			for _, p := range path {
				sid, ok := SocketOfPort(plan, p)
				if !ok {
					continue
				}
				if sid == d {
					after = true
					continue
				}
				if after {
					included[sid] = true
				}
			}
		}
	}

	out := make([]SocketID, 0, len(included))
	for s := range included {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
