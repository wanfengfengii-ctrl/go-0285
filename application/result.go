package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"precast-wall-grout-support-release/domain"
)

// Result is the normalized outcome of a successful command. It carries the
// task aggregate version and the logical time at which the command committed.
type Result struct {
	TaskID            domain.TaskID      `json:"taskId"`
	Stage             domain.Stage       `json:"stage"`
	Generation        domain.Generation  `json:"generation"`
	AggregateVersion  int64              `json:"aggregateVersion"`
	LogicalTime       domain.LogicalTime `json:"logicalTime"`
	ReleaseCredential string             `json:"releaseCredential,omitempty"`
}

// ContentDigest computes the normalized content digest of a command for
// idempotency. It hashes the command's domain-relevant fields so equal content
// yields an equal digest and different content yields a different one.
func ContentDigest(cmd Command) (string, error) {
	payload := struct {
		Type             CommandType         `json:"type"`
		TaskID           domain.TaskID       `json:"taskId"`
		Generation       domain.Generation   `json:"generation"`
		Building         string              `json:"building"`
		Level            string              `json:"level"`
		WallPanel        string              `json:"wallPanel"`
		CatalogVer       int64               `json:"catalogVersion"`
		RuleDigest       string              `json:"ruleDigest"`
		Connections      []ConnectionSpec    `json:"connections"`
		PortNodes        []PortNodeSpec      `json:"portNodes"`
		PortEdges        []PortEdgeSpec      `json:"portEdges"`
		SlurryPaths      [][]domain.PortID   `json:"slurryPaths"`
		MaterialBatch    string              `json:"materialBatch"`
		WaterBatch       string              `json:"waterBatch"`
		VolumeML         int64               `json:"theoreticalVolumeMl"`
		LossCeiling      int64               `json:"lossCeilingPpm"`
		Specimens        []SpecimenSpec      `json:"specimens"`
		UltrasonicLines  []string            `json:"ultrasonicLines"`
		EndoscopeHoles   []string            `json:"endoscopeHoles"`
		ReleaseThreshold int64               `json:"releaseThreshold"`
		BatchID          string              `json:"batchId"`
		InputGrams       int64               `json:"inputGrams"`
		WaterML          int64               `json:"waterMl"`
		LossML           int64               `json:"lossMl"`
		SampleML         int64               `json:"sampleMl"`
		WorkTicks        int64               `json:"workTicks"`
		ResourceType     domain.ResourceType `json:"resourceType"`
		ResourceID       string              `json:"resourceId"`
		LeaseTicks       int64               `json:"leaseTicks"`
		Volume           int64               `json:"volumeMl"`
		PortID           domain.PortID       `json:"portId"`
		SocketID         domain.SocketID     `json:"socketId"`
		Pressure         int64               `json:"pressure"`
		SpecimenID       string              `json:"specimenId"`
		Value            int64               `json:"value"`
		Reason           string              `json:"reason"`
		Defects          []domain.SocketID   `json:"defects"`
		EvidenceDigest   string              `json:"evidenceDigest"`
		TerminalType     domain.TerminalType `json:"terminalType"`
	}{
		Type: cmd.Type, TaskID: cmd.TaskID, Generation: cmd.Generation,
		Building: cmd.Building, Level: cmd.Level, WallPanel: cmd.WallPanel,
		CatalogVer: cmd.CatalogVersion, RuleDigest: cmd.RuleDigest,
		Connections: cmd.Connections, PortNodes: cmd.PortNodes, PortEdges: cmd.PortEdges,
		SlurryPaths: cmd.SlurryPaths, MaterialBatch: cmd.MaterialBatch, WaterBatch: cmd.WaterBatch,
		VolumeML: cmd.TheoreticalVolumeML, LossCeiling: cmd.LossCeilingPPM, Specimens: cmd.Specimens,
		UltrasonicLines: cmd.UltrasonicLines, EndoscopeHoles: cmd.EndoscopeHoles,
		ReleaseThreshold: cmd.ReleaseThreshold, BatchID: cmd.BatchID, InputGrams: cmd.InputGrams,
		WaterML: cmd.WaterML, LossML: cmd.LossML, SampleML: cmd.SampleML, WorkTicks: cmd.WorkTicks,
		ResourceType: cmd.ResourceType, ResourceID: cmd.ResourceID, LeaseTicks: cmd.LeaseTicks,
		Volume: cmd.VolumeML, PortID: cmd.PortID, SocketID: cmd.SocketID, Pressure: cmd.Pressure,
		SpecimenID: cmd.SpecimenID, Value: cmd.Value, Reason: cmd.Reason, Defects: cmd.Defects,
		EvidenceDigest: cmd.EvidenceDigest, TerminalType: cmd.TerminalType,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
