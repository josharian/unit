package unit_test

import (
	"testing"

	"github.com/dsnet/try"
	"github.com/josharian/unit"
)

func TestConvert(t *testing.T) {
	try.F(t.Fatal)
	s := unit.NewSystem("test")
	try.E(unit.AddBasic(s, "m"))
	try.E(unit.AddBasic(s, "s"))
	try.E(unit.AddConversion(s, "m", "km", 1000))
	try.E(unit.AddConversion(s, "km", "gm", 1000))
	type meter float64
	type kilometer float64
	type gigameter float64
	type seconds float64
	type metersPerSecond float64
	type secondsPerMeter float64
	type metersSquaredPerSecond float64
	try.E(unit.AddType[meter](s, []string{"m"}, nil))
	try.E(unit.AddType[kilometer](s, []string{"km"}, nil))
	try.E(unit.AddType[gigameter](s, []string{"gm"}, nil))
	try.E(unit.AddType[seconds](s, []string{"s"}, nil))
	try.E(unit.AddType[metersPerSecond](s, []string{"m"}, []string{"s"}))
	try.E(unit.AddType[secondsPerMeter](s, []string{"s"}, []string{"m"}))
	try.E(unit.AddType[metersSquaredPerSecond](s, []string{"m", "m"}, []string{"s"}))

	m := meter(5000)
	km := try.E1(unit.Convert[kilometer](s, m))
	if km != 5 {
		t.Fatalf("5000km = %vm, want 5", km)
	}
	gm := try.E1(unit.Convert[gigameter](s, m))
	if gm != 5/1000.0 {
		t.Fatalf("5000km = %vgm, want 5/1000.0", gm)
	}
	m = try.E1(unit.Convert[meter](s, km))
	if m != 5000 {
		t.Fatalf("5km = %vm, want 5000", m)
	}

	{
		var m meter = 10
		var ms metersPerSecond = 25
		mss := try.E1(unit.Combine[metersSquaredPerSecond](s, m, ms))
		if mss != 250 {
			t.Fatalf("want 250 m*m/s, got %v", mss)
		}
	}

	{
		var km kilometer = 10
		var ms metersPerSecond = 25
		mss := try.E1(unit.Combine[metersSquaredPerSecond](s, km, ms))
		if mss != 250000 {
			t.Fatalf("want 250000 m*m/s, got %v", mss)
		}
	}

	{
		var m meter = 10
		var sm secondsPerMeter = 25
		mss := try.E1(unit.Combine[metersSquaredPerSecond](s, m, sm))
		const want = 10 / 25.0
		if mss != want {
			t.Fatalf("want %v m*m/s, got %v", want, mss)
		}
	}
}
