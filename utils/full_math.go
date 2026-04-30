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
func (m *FullMath) MulDivV2(x, y, denominator, result, remainder *uint256.Int) error {
	if x.IsZero() || y.IsZero() || denominator.IsZero() {
		result.Clear()
		if remainder != nil {
			remainder.Clear()
		}
		return nil
	}
	// Быстрый путь: x[0]=0 (числитель = liquidity<<96) и y[3]=0 (160-битный операнд).
	// 9 Mul64 вместо 16 для полного умножения.
	var p [8]uint64
	if x[0] == 0 && y[3] == 0 {
		p = umul_lo3(x, y)
	} else {
		p = umul(x, y)
	}

	m.quot[7], m.quot[6], m.quot[5], m.quot[4], m.quot[3], m.quot[2], m.quot[1], m.quot[0] = 0, 0, 0, 0, 0, 0, 0, 0
	// Если caller просит remainder — пишем его напрямую в udivrem, минуя
	// внутренний m.rem и лишнюю копию (`Set` = 4-word memcopy).
	remDst := m.rem
	if remainder != nil {
		remDst = remainder
	}
	m.u256utils.udivrem(m.quot[:], p[:], denominator, remDst)

	// copy(z[:], m.quot[:4])
	result[0], result[1], result[2], result[3] = m.quot[0], m.quot[1], m.quot[2], m.quot[3]

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

// DivInto вычисляет result = floor(a / denominator) через наш pre-allocated udivrem.
// Заменяет holiman.Div/DivMod, избегая wrapper-оверхеда:
//   - проверок Gt/Lt (≈0.5s + 0.55s в профиле на всех вызовах)
//   - стек-аллокации var quot, rem Int (64 байта)
//   - двух Set-копий (8 слов каждая)
//
// Остаток сохраняется в m.rem.
// Пресловажно: denominator != 0; a < d обрабатывается корректно (result = 0, rem = a).
func (m *FullMath) DivInto(a, denominator, result *uint256.Int) {
	m.quot[0], m.quot[1], m.quot[2], m.quot[3] = 0, 0, 0, 0
	m.u256utils.udivrem(m.quot[:4], a[:], denominator, m.rem)
	result[0], result[1], result[2], result[3] = m.quot[0], m.quot[1], m.quot[2], m.quot[3]
}

// DivRoundingUp Returns ceil(x / y)
func (m *FullMath) DivRoundingUp(a, denominator, result *uint256.Int) {
	m.DivInto(a, denominator, result)
	if !m.rem.IsZero() {
		result.AddUint64(result, 1)
	}
}
