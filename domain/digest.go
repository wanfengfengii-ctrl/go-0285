package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CatalogDigest computes a stable rule digest over the catalog's authoritative
// fields. Locking pins this digest so a stale catalog (older version or a
// different rule set) is detected with STALE_RULE_DIGEST.
func CatalogDigest(c CatalogSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "v%d\n", c.Version)
	fmt.Fprintf(&b, "water:%d:%d\n", c.WaterRules.MinRatioPPM, c.WaterRules.MaxRatioPPM)
	fmt.Fprintf(&b, "loss:%d:%d:%d\n", c.LossBounds.MaxLossRatioPPM, c.LossBounds.MinVolumeML, c.LossBounds.MaxVolumeML)
	fmt.Fprintf(&b, "work:%d:%d\n", c.WorkLimits.MinWorkTicks, c.WorkLimits.MaxWorkTicks)

	compat := make([]string, 0, len(c.Compat))
	for _, sc := range c.Compat {
		compat = append(compat, sc.RebarSpec+"|"+sc.SleeveSpec)
	}
	sort.Strings(compat)
	for _, s := range compat {
		fmt.Fprintf(&b, "compat:%s\n", s)
	}

	certs := make([]string, 0, len(c.MaterialCert))
	for _, mc := range c.MaterialCert {
		certs = append(certs, mc.BatchID)
	}
	sort.Strings(certs)
	for _, s := range certs {
		fmt.Fprintf(&b, "cert:%s\n", s)
	}

	persons := make([]string, 0, len(c.Personnel))
	for _, p := range c.Personnel {
		persons = append(persons, fmt.Sprintf("%s:%t", p.PersonID, p.Qualified))
	}
	sort.Strings(persons)
	for _, s := range persons {
		fmt.Fprintf(&b, "person:%s\n", s)
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
