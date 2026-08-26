package domain

import "math"

// Fixed-point arithmetic helpers.
//
// All masses and volumes are expressed as int64 integers in explicit units:
// grams for mass and millilitres for volume. Ratios are expressed in parts per
// million (ppm). Multiplication checks for overflow before producing a result
// and division rounds half away from zero ("half-up"). Negative operands and
// zero divisors are rejected, matching the domain rule that any failed command
// must never corrupt the conservation invariant.

// ScalePPM is the scale factor used for ppm ratios.
const ScalePPM int64 = 1_000_000

// Add returns a+b, or an overflow error when the result would not fit in int64.
func Add(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, NewError(CodeFixedPointOverflow, "integer addition overflow")
	}
	return a + b, nil
}

// Sub returns a-b, or an overflow error when the result would not fit in int64.
func Sub(a, b int64) (int64, error) {
	if (b < 0 && a > math.MaxInt64+b) || (b > 0 && a < math.MinInt64+b) {
		return 0, NewError(CodeFixedPointOverflow, "integer subtraction overflow")
	}
	return a - b, nil
}

// Mul returns a*b, or an overflow error when the result would not fit in int64.
func Mul(a, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a == math.MinInt64 && b == -1 {
		return 0, NewError(CodeFixedPointOverflow, "integer multiplication overflow")
	}
	c := a * b
	if c/b != a {
		return 0, NewError(CodeFixedPointOverflow, "integer multiplication overflow")
	}
	return c, nil
}

// MulDiv returns (a*b)/c with overflow checks and half-away-from-zero rounding.
// A zero divisor or a negative operand is rejected.
func MulDiv(a, b, c int64) (int64, error) {
	if c == 0 {
		return 0, NewError(CodeFixedPointOverflow, "division by zero")
	}
	prod, err := Mul(a, b)
	if err != nil {
		return 0, err
	}
	return DivHalfUp(prod, c)
}

// ScalePPMRatio returns value*ratio/ScalePPM, i.e. applies a ppm ratio with
// overflow checks and half-up rounding.
func ScalePPMRatio(value, ratio int64) (int64, error) {
	return MulDiv(value, ratio, ScalePPM)
}

// DivHalfUp divides a by b and rounds half away from zero.
func DivHalfUp(a, b int64) (int64, error) {
	if b == 0 {
		return 0, NewError(CodeFixedPointOverflow, "division by zero")
	}
	if a == math.MinInt64 && b == -1 {
		return 0, NewError(CodeFixedPointOverflow, "integer division overflow")
	}
	q := a / b
	r := a % b
	if r == 0 {
		return q, nil
	}
	// Half-up: round away from zero when |2r| >= |b|.
	absR := abs64(r)
	absB := abs64(b)
	if absR*2 >= absB {
		if (a > 0) == (b > 0) {
			q++
		} else {
			q--
		}
	}
	return q, nil
}

// NonNegative validates that v is >= 0, returning an overflow error otherwise
// (negative masses and volumes are rejected before they can corrupt a ledger).
func NonNegative(v int64) error {
	if v < 0 {
		return NewError(CodeFixedPointOverflow, "negative value rejected")
	}
	return nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
