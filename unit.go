// Package unit provides unit-safe calculations.
//
// It is focused on correctness, not performance.
package unit

import (
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
)

// System is a unified system of units.
// Because generics doesn't support methods,
// all manipulation of Systems go through functions.
type System struct {
	name  string               // optional, printed in errors
	typOf map[reflect.Type]dim // Go type => associated unit
	root  map[string]basic     // unit name => root unit
}

// NewSystem creates a new units system named name.
// The name is used in error strings.
//
// Unit systems must be constructed in a particular order:
// Add all basic units and conversions, then add all types, then use.
// Once all additions are completed, a unit system is concurrency-safe.
func NewSystem(name string) *System {
	return &System{name: name, typOf: make(map[reflect.Type]dim), root: make(map[string]basic)}
}

type basic struct {
	name   string
	idx    int
	factor *big.Rat
}

// dim is a dimensional unit.
type dim struct {
	num    []string
	den    []string
	factor *big.Rat
	vec    []int
}

func (d dim) String() string {
	return fmt.Sprintf("%v * %v / %v", d.factor, strings.Join(d.num, "*"), strings.Join(d.den, "*"))
}

func convertible(d, e dim) bool {
	return slices.Equal(d.num, e.num) && slices.Equal(d.den, e.den)
}

func newRat(f float64) *big.Rat {
	return new(big.Rat).SetFloat64(f)
}

func newRatAny(x any) *big.Rat {
	return newRat(reflect.ValueOf(x).Float())
}

// AddBasic adds a basic unit to s.
// Example: AddBasic(s, "meter")
// If s already has a unit named name, AddBasic returns an error.
func AddBasic(s *System, name string) error {
	if _, ok := s.root[name]; ok {
		return fmt.Errorf("%v already has a unit named %q", s.name, name)
	}
	s.root[name] = basic{name: name, factor: big.NewRat(1, 1), idx: len(s.root)}
	return nil
}

// AddConversion adds a conversion between basic units: from * factor = to.
// If s has no unit named from, or already has a unit named to, AddConversion returns an error.
// Example: AddConversion(s, "meter", "kilometer", 1000)
func AddConversion(s *System, from, to string, factor float64) error {
	f, ok := s.root[from]
	if !ok {
		return fmt.Errorf("%v has no unit named %q", s.name, from)
	}
	if _, ok := s.root[to]; ok {
		return fmt.Errorf("%v already has a unit named %q", s.name, to)
	}
	s.root[to] = basic{name: from, factor: newRat(factor), idx: f.idx}
	return nil
}

// AddType associates T with a dimensional unit, with the specified numerator and denominator units.
// If s already associates T with a unit, or the unit can be simplified (e.g. "meters per foot"),
// or any units in num or den are unrecognized, AddType returns an error.
//
// Example: type meter float64; AddType[meter](s, []string{"meter"}, nil)
// Example: type mpg float64; AddType[mile](s, []string{"mile"}, []string{"gallon"})
// Example: type metersPerSecondSquared float64; AddType[metersPerSecondSquared](s, []string{"m"}, []string{"s", "s"})
func AddType[T ~float64](s *System, num, den []string) error {
	var t T
	rt := reflect.TypeOf(t)
	if _, ok := s.typOf[rt]; ok {
		return fmt.Errorf("%v has a unit associated with type %v", s.name, rt)
	}
	// Simplify and canonicalize.
	factor := newRat(1)
	vec := make([]int, len(s.root))
	isNum := make(map[string]string)
	var canonNum []string
	for _, n := range num {
		root, ok := s.root[n]
		if !ok {
			return fmt.Errorf("%v has no unit named %q", s.name, n)
		}
		canonNum = append(canonNum, root.name)
		factor = factor.Mul(factor, root.factor)
		isNum[root.name] = n
		vec[root.idx]++
	}
	var canonDen []string
	for _, d := range den {
		root, ok := s.root[d]
		if !ok {
			return fmt.Errorf("%v has no unit named %q", s.name, d)
		}
		if n, ok := isNum[root.name]; ok {
			return fmt.Errorf("unit can be simplified: numerator %q and denominator %q share common unit %q", n, d, root.name)
		}
		canonDen = append(canonDen, root.name)
		factor = factor.Quo(factor, root.factor)
		vec[root.idx]--
	}
	sort.Strings(canonNum)
	sort.Strings(canonDen)
	s.typOf[rt] = dim{num: canonNum, den: canonDen, factor: factor, vec: vec}
	return nil
}

// Convert converts from from to To.
// Both from and To must have had their types added to s via an AddType call.
func Convert[To ~float64](s *System, from any) (To, error) {
	var to To
	toTyp := reflect.TypeOf(to)
	toDim, ok := s.typOf[toTyp]
	if !ok {
		return to, fmt.Errorf("%s has no unit associated with type %v", s.name, toTyp)
	}
	fromTyp := reflect.TypeOf(from)
	fromDim, ok := s.typOf[fromTyp]
	if !ok {
		return to, fmt.Errorf("%s has no unit associated with type %v", s.name, fromTyp)
	}
	if !convertible(toDim, fromDim) {
		return to, fmt.Errorf("%s cannot convert from %v to %v", s.name, fromTyp, toTyp)
	}
	result := newRatAny(from)
	result = result.Quo(result, toDim.factor)
	result = result.Mul(result, fromDim.factor)
	f, _ := result.Float64()
	return To(f), nil
}

// Combine combines the values in args to get a value with the units To.
// It will only do the computation if the units make the result unambiguous.
// For example, if you request meters per second, and provide m meters
// and s seconds, it will result m/s.
func Combine[To ~float64](s *System, args ...any) (To, error) {
	var to To
	toTyp := reflect.TypeOf(to)
	toDim, ok := s.typOf[toTyp]
	if !ok {
		return to, fmt.Errorf("%s has no unit associated with type %v", s.name, toTyp)
	}

	veclen := len(s.root)
	var vecs [][]int
	var factors []*big.Rat
	for _, arg := range args {
		argTyp := reflect.TypeOf(arg)
		argDim, ok := s.typOf[argTyp]
		if !ok {
			return to, fmt.Errorf("%s has no unit associated with type %v", s.name, argTyp)
		}
		if len(argDim.vec) != veclen {
			return to, fmt.Errorf("%s was constructed out of order, please see unit.NewSystem docs", s.name)
		}
		vecs = append(vecs, argDim.vec)
		factors = append(factors, argDim.factor)
	}
	if len(vecs) > 16 {
		return to, fmt.Errorf("too many arguments to Combine, max is 16, got %d", len(vecs))
	}
	bits, found, ambiguous := solve(vecs, toDim.vec)
	if ambiguous {
		return to, fmt.Errorf("ambiguous conversion") // TODO: better error
	}
	if !found {
		return to, fmt.Errorf("impossible conversion") // TODO: better error
	}
	result := newRat(1)
	for i := range vecs {
		factor := factors[i]
		val := newRatAny(args[i])
		if bits.at(i) == -1 {
			// Divide
			result = result.Quo(result, val)
			result = result.Quo(result, factor)
		} else {
			// Multiply
			result = result.Mul(result, val)
			result = result.Mul(result, factor)
		}
	}
	result = result.Quo(result, toDim.factor)
	f, _ := result.Float64()
	return To(f), nil
}

// solve finds exactly one combination of inputs that generates out.
// If it doesn't find any, target is nil.
// If it finds more than one, unambiguous is false.
//
// Assume in has length N and inner length X.
// Out must have length X as well.
// The targets we are seeking are slices of length N such that:
// (a) every element of target is -1 or 1 and
// (b) for all 0 <= x < X, output[x] = sum over 0 <= n < N of input[n][x] * target[n].
func solve(in [][]int, out []int) (target bitset32, found, ambiguous bool) {
	n := len(in)
	if n > 16 {
		panic("solve: too big")
	}
	x := len(out)
	sum := make([]int, x)
	// Do this the stupid, exponential way.
	// There's probably a better way. I don't know it.
NextBits:
	for bits := bitset32(0); bits < 1<<n; bits++ {
		copy(sum, out)
		for i, vec := range in {
			mul := bits.at(i)
			for x, v := range vec {
				sum[x] += -1 * v * mul
			}
		}
		for _, s := range sum {
			if s != 0 {
				continue NextBits
			}
		}
		if found {
			return 0, true, true
		}
		target = bits
		found = true
	}
	if found {
		return target, true, false
	}
	return 0, false, false
}

type bitset32 uint32

func (b bitset32) at(idx int) int {
	if b&(1<<idx) != 0 {
		return 1
	}
	return -1
}
