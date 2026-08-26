package application

import "precast-wall-grout-support-release/domain"

// DefaultCatalog builds a small, self-consistent rules catalog used by the
// production entry point and the smoke script. Tests construct their own
// catalogs to exercise specific compatibility and boundary cases.
func DefaultCatalog() domain.CatalogSnapshot {
	return domain.CatalogSnapshot{
		Version: 1,
		Compat: []domain.SocketCompat{
			{RebarSpec: "HRB400-12", SleeveSpec: "GT-12"},
			{RebarSpec: "HRB400-16", SleeveSpec: "GT-16"},
			{RebarSpec: "HRB400-20", SleeveSpec: "GT-20"},
		},
		MaterialCert: []domain.MaterialCert{
			{BatchID: "MAT-001", Version: 1},
			{BatchID: "WAT-001", Version: 1},
		},
		WaterRules: domain.WaterRule{MinRatioPPM: 300000, MaxRatioPPM: 500000},
		LossBounds: domain.LossBounds{
			MaxLossRatioPPM: 50000,
			MinVolumeML:     1000,
			MaxVolumeML:     100000,
		},
		WorkLimits: domain.WorkLimits{MinWorkTicks: 10, MaxWorkTicks: 1000},
		Personnel: []domain.PersonnelQualification{
			{PersonID: "reviewer-a", Qualified: true, ValidFrom: 0, ValidUntil: 1 << 40},
			{PersonID: "reviewer-b", Qualified: true, ValidFrom: 0, ValidUntil: 1 << 40},
		},
	}
}
