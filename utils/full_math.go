package utils

import (
	"errors"
	"math/big"

	"github.com/holiman/uint256"
)

var (
	ErrMulDivOverflow = errors.New("muldiv overflow")
	One               = big.NewInt(1)
)

type FullMath struct {
	rem       *uint256.Int
	result    *uint256.Int
	remainder *Uint256
}

func NewFullMath() *FullMath {
	return &FullMath{
		rem:       new(uint256.Int),
		result:    new(uint256.Int),
		remainder: new(Uint256),
	}
}

// MulDivRoundingUp Calculates ceil(a×b÷denominator) with full precision
func (m *FullMath) MulDivRoundingUp(a, b, denominator *uint256.Int) (*uint256.Int, error) {
	return m.result, m.MulDivRoundingUpV2(a, b, denominator, m.result)
}

func (m *FullMath) MulDivRoundingUpV2(a, b, denominator, result *uint256.Int) error {
	m.remainder.Clear()

	if err := m.MulDivV2(a, b, denominator, result, m.remainder); err != nil {
		return err
	}

	if !m.remainder.IsZero() {
		if result.Eq(MaxUint256) {
			return ErrInvariant
		}

		result.AddUint64(result, 1)
	}

	return nil
}

// MulDivV2 z=floor(a×b÷denominator), r=a×b%denominator
// (pass remainder=nil if not required)
// (the main usage for `remainder` is to be used in `MulDivRoundingUpV2` to determine if we need to round up, so it won't have to call MulMod again)
func (m *FullMath) MulDivV2(x, y, d, z, r *uint256.Int) error {
	if x.IsZero() || y.IsZero() || d.IsZero() {
		z.Clear()
		return nil
	}
	p := umul(x, y)

	var quot [8]uint64
	rem := udivrem(quot[:], p[:], d)
	if r != nil {
		r.Set(&rem)
	}

	copy(z[:], quot[:4])

	if (quot[4] | quot[5] | quot[6] | quot[7]) != 0 {
		return ErrMulDivOverflow
	}
	return nil
}

// MulDiv Calculates floor(a×b÷denominator) with full precision
func (m *FullMath) MulDiv(a, b, denominator *uint256.Int) (*uint256.Int, error) {
	var overflow bool

	if m.result, overflow = m.result.MulDivOverflow(a, b, denominator); overflow {
		return nil, ErrMulDivOverflow
	}

	return m.result, nil
}

// DivRoundingUp Returns ceil(x / y)
func (m *FullMath) DivRoundingUp(a, denominator, result *uint256.Int) {
	result.DivMod(a, denominator, m.rem)
	if !m.rem.IsZero() {
		result.AddUint64(result, 1)
	}
}
