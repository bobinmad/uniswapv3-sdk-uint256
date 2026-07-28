package utils

import (
	"errors"
	"math/big"
	"math/bits"

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

	// Однословная liquidity — основной делитель при расчёте fee в
	// defisimulator. Храним отдельный компактный cache, чтобы DivInto не
	// проходил общий udivrem с поиском длины и materialization unStorage.
	lastUint64Divisor uint64
	lastUint64Norm    uint64
	lastUint64Recip   uint64
	lastUint64Shift   uint
	hasUint64Divisor  bool
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
	if denominator[1]|denominator[2]|denominator[3] == 0 {
		m.divByUint64Into(a, denominator[0], result)
		return
	}

	m.quot[0], m.quot[1], m.quot[2], m.quot[3] = 0, 0, 0, 0
	m.u256utils.udivrem(m.quot[:4], a[:], denominator, m.rem)
	result[0], result[1], result[2], result[3] = m.quot[0], m.quot[1], m.quot[2], m.quot[3]
}

// divByUint64Into — specialization DivInto для однословного denominator.
// Нормализация и reciprocal кэшируются по последнему делителю; нормализованный
// dividend держится в локальных переменных, без записи/чтения unStorage.
//
// result может совпадать с a: исходные limbs читаются до записи результата.
func (m *FullMath) divByUint64Into(a *uint256.Int, denominator uint64, result *uint256.Int) {
	shift, normalized, reciprocal := m.lastUint64Shift, m.lastUint64Norm, m.lastUint64Recip
	if !m.hasUint64Divisor || m.lastUint64Divisor != denominator {
		shift = uint(bits.LeadingZeros64(denominator))
		normalized = denominator << shift
		reciprocal = reciprocal2by1(normalized)
		m.lastUint64Divisor = denominator
		m.lastUint64Shift = shift
		m.lastUint64Norm = normalized
		m.lastUint64Recip = reciprocal
		m.hasUint64Divisor = true
	}

	rshift := 64 - shift
	a0, a1, a2, a3 := a[0], a[1], a[2], a[3]
	var q0, q1, q2, q3, rem uint64

	switch {
	case a3 != 0:
		un4 := a3 >> rshift
		un3 := (a3 << shift) | (a2 >> rshift)
		un2 := (a2 << shift) | (a1 >> rshift)
		un1 := (a1 << shift) | (a0 >> rshift)
		un0 := a0 << shift
		if un4 == 0 && un3 < normalized {
			rem = un3
		} else {
			q3, rem = udivrem2by1(un4, un3, normalized, reciprocal)
		}
		q2, rem = udivrem2by1(rem, un2, normalized, reciprocal)
		q1, rem = udivrem2by1(rem, un1, normalized, reciprocal)
		q0, rem = udivrem2by1(rem, un0, normalized, reciprocal)

	case a2 != 0:
		un3 := a2 >> rshift
		un2 := (a2 << shift) | (a1 >> rshift)
		un1 := (a1 << shift) | (a0 >> rshift)
		un0 := a0 << shift
		if un3 == 0 && un2 < normalized {
			rem = un2
		} else {
			q2, rem = udivrem2by1(un3, un2, normalized, reciprocal)
		}
		q1, rem = udivrem2by1(rem, un1, normalized, reciprocal)
		q0, rem = udivrem2by1(rem, un0, normalized, reciprocal)

	case a1 != 0:
		un2 := a1 >> rshift
		un1 := (a1 << shift) | (a0 >> rshift)
		un0 := a0 << shift
		if un2 == 0 && un1 < normalized {
			rem = un1
		} else {
			q1, rem = udivrem2by1(un2, un1, normalized, reciprocal)
		}
		q0, rem = udivrem2by1(rem, un0, normalized, reciprocal)

	default:
		un1 := a0 >> rshift
		un0 := a0 << shift
		q0, rem = udivrem2by1(un1, un0, normalized, reciprocal)
	}

	result[0], result[1], result[2], result[3] = q0, q1, q2, q3
	m.rem[0], m.rem[1], m.rem[2], m.rem[3] = rem>>shift, 0, 0, 0
}

// DivRoundingUp Returns ceil(x / y)
func (m *FullMath) DivRoundingUp(a, denominator, result *uint256.Int) {
	m.DivInto(a, denominator, result)
	if !m.rem.IsZero() {
		result.AddUint64(result, 1)
	}
}
