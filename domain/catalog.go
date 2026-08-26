package domain

// CatalogSnapshot is a versioned, digest-identified, read-only snapshot of the
// reinforcement-sleeve compatibility and material rules catalog. Locking a
// task pins the catalog version and digest so a stale catalog is detected and
// rejected with STALE_RULE_DIGEST.
type CatalogSnapshot struct {
	Version      int64
	Digest       string
	Compat       []SocketCompat
	MaterialCert []MaterialCert
	WaterRules   WaterRule
	LossBounds   LossBounds
	WorkLimits   WorkLimits
	Personnel    []PersonnelQualification
}

// SocketCompat declares that a reinforcement bar of RebarSpec is compatible with
// a sleeve of SleeveSpec.
type SocketCompat struct {
	RebarSpec  string
	SleeveSpec string
}

// MaterialCert records a material batch proof with its version.
type MaterialCert struct {
	BatchID string
	Version int64
}

// WaterRule constrains water usage for a mix.
type WaterRule struct {
	MinRatioPPM int64
	MaxRatioPPM int64
}

// LossBounds constrains mixing loss and theoretical volume boundaries.
type LossBounds struct {
	MaxLossRatioPPM int64
	MinVolumeML     int64
	MaxVolumeML     int64
}

// WorkLimits bounds the working time (in logical ticks) of a mix batch.
type WorkLimits struct {
	MinWorkTicks int64
	MaxWorkTicks int64
}

// PersonnelQualification records a person's id and whether they hold a valid
// qualification.
type PersonnelQualification struct {
	PersonID   string
	Qualified  bool
	ValidFrom  LogicalTime
	ValidUntil LogicalTime
}

// IsValid reports whether the qualification holds at the given logical time.
func (p PersonnelQualification) IsValid(at LogicalTime) bool {
	return p.Qualified && at >= p.ValidFrom && at < p.ValidUntil
}
