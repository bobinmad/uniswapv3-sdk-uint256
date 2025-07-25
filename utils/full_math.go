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
	u256utils   *Uint256Utils
	rem, result *uint256.Int
	remainder   *Uint256
	quot        [8]uint64
}

func NewFullMath() *FullMath {
	return &FullMath{
		u256utils: NewUint256Utils(),
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
	if err := m.MulDivV2(a, b, denominator, result, m.remainder.Clear()); err != nil {
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

	// m.quot = [8]uint64{}
	m.quot[7], m.quot[6], m.quot[5], m.quot[4], m.quot[3], m.quot[2], m.quot[1], m.quot[0] = 0, 0, 0, 0, 0, 0, 0, 0
	m.u256utils.udivrem(m.quot[:], p[:], d, m.rem)
	if r != nil {
		r.Set(m.rem)
	}

	// copy(z[:], m.quot[:4])
	z[0], z[1], z[2], z[3] = m.quot[0], m.quot[1], m.quot[2], m.quot[3]

	if (m.quot[4] | m.quot[5] | m.quot[6] | m.quot[7]) != 0 {
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
