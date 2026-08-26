package domain

import "testing"

func TestAddSub(t *testing.T) {
	tests := []struct {
		name string
		a, b int64
		want int64
		err  bool
	}{
		{"add basic", 2, 3, 5, false},
		{"add zero", 0, 7, 7, false},
		{"sub basic", 10, 3, 7, false},
		{"sub negative result", 3, 10, -7, false},
		{"add overflow", maxInt, 1, 0, true},
		{"sub overflow", minInt, 1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got int64
			var err error
			if tt.name[0] == 'a' {
				got, err = Add(tt.a, tt.b)
			} else {
				got, err = Sub(tt.a, tt.b)
			}
			if tt.err {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				if !IsCode(err, CodeFixedPointOverflow) {
					t.Fatalf("expected FIXED_POINT_OVERFLOW, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}

func TestMulOverflow(t *testing.T) {
	if _, err := Mul(maxInt, 2); !IsCode(err, CodeFixedPointOverflow) {
		t.Fatalf("expected overflow, got %v", err)
	}
	if got, err := Mul(6, 7); err != nil || got != 42 {
		t.Fatalf("got %d err %v", got, err)
	}
	if got, err := Mul(0, 999); err != nil || got != 0 {
		t.Fatalf("got %d err %v", got, err)
	}
}

func TestMulDivHalfUp(t *testing.T) {
	tests := []struct {
		name    string
		a, b, c int64
		want    int64
		wantErr bool
	}{
		{"exact", 3, 4, 2, 6, false},
		{"half up", 1, 1, 2, 1, false},                      // 0.5 -> 1
		{"below half", 1, 1, 3, 0, false},                   // 0.333 -> 0
		{"scale ppm half up", 500000, 1, 1000000, 1, false}, // 0.5 -> 1
		{"div by zero", 1, 1, 0, 0, true},
		{"mul overflow", maxInt, 2, 1, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MulDiv(tt.a, tt.b, tt.c)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				if !IsCode(err, CodeFixedPointOverflow) {
					t.Fatalf("expected FIXED_POINT_OVERFLOW, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}

func TestScalePPMRatio(t *testing.T) {
	// 1000ml * 50000ppm = 50ml
	got, err := ScalePPMRatio(1000, 50000)
	if err != nil || got != 50 {
		t.Fatalf("got %d err %v", got, err)
	}
}

func TestDivHalfUpNegative(t *testing.T) {
	// Rounding away from zero: -1/2 = -0.5 -> -1
	got, err := DivHalfUp(-1, 2)
	if err != nil || got != -1 {
		t.Fatalf("got %d err %v", got, err)
	}
}

func TestNonNegative(t *testing.T) {
	if err := NonNegative(5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := NonNegative(-1); !IsCode(err, CodeFixedPointOverflow) {
		t.Fatalf("expected rejection, got %v", err)
	}
}

const (
	maxInt = int64(^uint64(0) >> 1)
	minInt = -maxInt - 1
)
