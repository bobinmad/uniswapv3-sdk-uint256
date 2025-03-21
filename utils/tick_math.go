package utils

import (
	"errors"
	"math/big"

	"github.com/KyberNetwork/int256"
	"github.com/holiman/uint256"
)

type TickCalculator struct {
	bitCalculator            *BitCalculator
	ratio, rem               *Uint256
	tmp                      *Uint256
	sqrtRatioX128            *Uint256
	r                        *Uint256
	f                        *Uint256
	logSqrt10001, tmp1, tmp2 *Int256
	sqrtRatio                *Uint160
	log2                     *int256.Int
}

func NewTickCalculator() *TickCalculator {
	return &TickCalculator{
		bitCalculator: NewBitCalculator(),
		ratio:         new(Uint256),
		rem:           new(Uint256),
		tmp:           new(Uint256),
		sqrtRatioX128: new(Uint256),
		r:             new(Uint256),
		f:             new(Uint256),
		logSqrt10001:  new(Int256),
		tmp1:          new(Int256),
		tmp2:          new(Int256),
		sqrtRatio:     new(Uint160),
		log2:          new(int256.Int),
	}
}

const (
	MinTick = -887272  // The minimum tick that can be used on any pool.
	MaxTick = -MinTick // The maximum tick that can be used on any pool.
)

var (
	Q32             = big.NewInt(1 << 32)
	MinSqrtRatio    = big.NewInt(4295128739)                                                          // The sqrt ratio corresponding to the minimum tick that could be used on any pool.
	MaxSqrtRatio, _ = new(big.Int).SetString("1461446703485210103287273052203988822378723970342", 10) // The sqrt ratio corresponding to the maximum tick that could be used on any pool.

	Q32U256          = uint256.NewInt(1 << 32)
	MinSqrtRatioU256 = uint256.NewInt(4295128739)                                                   // The sqrt ratio corresponding to the minimum tick that could be used on any pool.
	MaxSqrtRatioU256 = uint256.MustFromDecimal("1461446703485210103287273052203988822378723970342") // The sqrt ratio corresponding to the maximum tick that could be used on any pool.
)

var (
	ErrInvalidTick      = errors.New("invalid tick")
	ErrInvalidSqrtRatio = errors.New("invalid sqrt ratio")
)

func (c *TickCalculator) mulShift(val *Uint256, mulBy *Uint256) {
	val.Rsh(c.tmp.Mul(val, mulBy), 128)
}

var (
	sqrtConst1  = uint256.MustFromHex("0xfffcb933bd6fad37aa2d162d1a594001")
	sqrtConst2  = uint256.MustFromHex("0x100000000000000000000000000000000")
	sqrtConst3  = uint256.MustFromHex("0xfff97272373d413259a46990580e213a")
	sqrtConst4  = uint256.MustFromHex("0xfff2e50f5f656932ef12357cf3c7fdcc")
	sqrtConst5  = uint256.MustFromHex("0xffe5caca7e10e4e61c3624eaa0941cd0")
	sqrtConst6  = uint256.MustFromHex("0xffcb9843d60f6159c9db58835c926644")
	sqrtConst7  = uint256.MustFromHex("0xff973b41fa98c081472e6896dfb254c0")
	sqrtConst8  = uint256.MustFromHex("0xff2ea16466c96a3843ec78b326b52861")
	sqrtConst9  = uint256.MustFromHex("0xfe5dee046a99a2a811c461f1969c3053")
	sqrtConst10 = uint256.MustFromHex("0xfcbe86c7900a88aedcffc83b479aa3a4")
	sqrtConst11 = uint256.MustFromHex("0xf987a7253ac413176f2b074cf7815e54")
	sqrtConst12 = uint256.MustFromHex("0xf3392b0822b70005940c7a398e4b70f3")
	sqrtConst13 = uint256.MustFromHex("0xe7159475a2c29b7443b29c7fa6e889d9")
	sqrtConst14 = uint256.MustFromHex("0xd097f3bdfd2022b8845ad8f792aa5825")
	sqrtConst15 = uint256.MustFromHex("0xa9f746462d870fdf8a65dc1f90e061e5")
	sqrtConst16 = uint256.MustFromHex("0x70d869a156d2a1b890bb3df62baf32f7")
	sqrtConst17 = uint256.MustFromHex("0x31be135f97d08fd981231505542fcfa6")
	sqrtConst18 = uint256.MustFromHex("0x9aa508b5b7a84e1c677de54f3e99bc9")
	sqrtConst19 = uint256.MustFromHex("0x5d6af8dedb81196699c329225ee604")
	sqrtConst20 = uint256.MustFromHex("0x2216e584f5fa1ea926041bedfe98")
	sqrtConst21 = uint256.MustFromHex("0x48a170391f7dc42444e8fa2")

	MaxUint256 = uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
)

// deprecated
func GetSqrtRatioAtTick(tick int) (*big.Int, error) {
	panic("GetSqrtRatioAtTick() is deprecated")
}

/**
 * Returns the sqrt ratio as a Q64.96 for the given tick. The sqrt ratio is computed as sqrt(1.0001)^tick
 * @param tick the tick for which to compute the sqrt ratio
 */
func (c *TickCalculator) GetSqrtRatioAtTickV2(tick int, result *Uint160) {
	// if tick < MinTick || tick > MaxTick {
	// 	return ErrInvalidTick
	// }

	if tick < 0 {
		tick = -tick
	}

	if tick&0x1 != 0 {
		c.ratio.Set(sqrtConst1)
	} else {
		c.ratio.Set(sqrtConst2)
	}
	if (tick & 0x2) != 0 {
		c.mulShift(c.ratio, sqrtConst3)
	}
	if (tick & 0x4) != 0 {
		c.mulShift(c.ratio, sqrtConst4)
	}
	if (tick & 0x8) != 0 {
		c.mulShift(c.ratio, sqrtConst5)
	}
	if (tick & 0x10) != 0 {
		c.mulShift(c.ratio, sqrtConst6)
	}
	if (tick & 0x20) != 0 {
		c.mulShift(c.ratio, sqrtConst7)
	}
	if (tick & 0x40) != 0 {
		c.mulShift(c.ratio, sqrtConst8)
	}
	if (tick & 0x80) != 0 {
		c.mulShift(c.ratio, sqrtConst9)
	}
	if (tick & 0x100) != 0 {
		c.mulShift(c.ratio, sqrtConst10)
	}
	if (tick & 0x200) != 0 {
		c.mulShift(c.ratio, sqrtConst11)
	}
	if (tick & 0x400) != 0 {
		c.mulShift(c.ratio, sqrtConst12)
	}
	if (tick & 0x800) != 0 {
		c.mulShift(c.ratio, sqrtConst13)
	}
	if (tick & 0x1000) != 0 {
		c.mulShift(c.ratio, sqrtConst14)
	}
	if (tick & 0x2000) != 0 {
		c.mulShift(c.ratio, sqrtConst15)
	}
	if (tick & 0x4000) != 0 {
		c.mulShift(c.ratio, sqrtConst16)
	}
	if (tick & 0x8000) != 0 {
		c.mulShift(c.ratio, sqrtConst17)
	}
	if (tick & 0x10000) != 0 {
		c.mulShift(c.ratio, sqrtConst18)
	}
	if (tick & 0x20000) != 0 {
		c.mulShift(c.ratio, sqrtConst19)
	}
	if (tick & 0x40000) != 0 {
		c.mulShift(c.ratio, sqrtConst20)
	}
	if (tick & 0x80000) != 0 {
		c.mulShift(c.ratio, sqrtConst21)
	}

	if tick > 0 {
		c.ratio.Set(result.Div(MaxUint256, c.ratio))
	}

	// back to Q96
	result.DivMod(c.ratio, Q32U256, c.rem)
	if !c.rem.IsZero() {
		result.AddUint64(result, 1)
	}
}

var (
	magicSqrt10001 = int256.MustFromDec("255738958999603826347141")
	magicTickLow   = int256.MustFromDec("3402992956809132418596140100660247210")
	magicTickHigh  = int256.MustFromDec("291339464771989622907027621153398088495")
)

// deprecated
func GetTickAtSqrtRatio(sqrtRatioX96 *big.Int) (int, error) {
	panic("GetTickAtSqrtRatio() is deprecated")
	// return GetTickAtSqrtRatioV2(uint256.MustFromBig(sqrtRatioX96))
}

/**
 * Returns the tick corresponding to a given sqrt ratio, s.t. #getSqrtRatioAtTick(tick) <= sqrtRatioX96
 * and #getSqrtRatioAtTick(tick + 1) > sqrtRatioX96
 * @param sqrtRatioX96 the sqrt ratio as a Q64.96 for which to compute the tick
 */
func (c *TickCalculator) GetTickAtSqrtRatioV2(sqrtRatioX96 *Uint160) (int, error) {
	if sqrtRatioX96.Lt(MinSqrtRatioU256) || !sqrtRatioX96.Lt(MaxSqrtRatioU256) {
		return 0, ErrInvalidSqrtRatio
	}

	c.sqrtRatioX128.Lsh(sqrtRatioX96, 32)
	msb, err := c.bitCalculator.MostSignificantBit(c.sqrtRatioX128)
	if err != nil {
		return 0, err
	}

	if msb >= 128 {
		c.r.Rsh(c.sqrtRatioX128, msb-127)
	} else {
		c.r.Lsh(c.sqrtRatioX128, 127-msb)
	}

	c.log2.Lsh(c.log2.SetInt64(int64(msb-128)), 64)

	for i := 0; i < 14; i++ {
		c.tmp.Mul(c.r, c.r)
		c.r.Rsh(c.tmp, 127)
		c.f.Rsh(c.r, 128)
		c.tmp.Lsh(c.f, uint(63-i))

		// this is for Or, so we can cast the underlying words directly without copying
		tmpsigned := (*int256.Int)(c.tmp)

		c.log2.Or(c.log2, tmpsigned)
		c.r.Rsh(c.r, uint(c.f.Uint64()))
	}

	c.logSqrt10001.Mul(c.log2, magicSqrt10001)

	tickLow := int(c.tmp2.Rsh(c.tmp1.Sub(c.logSqrt10001, magicTickLow), 128).Uint64())
	tickHigh := int(c.tmp2.Rsh(c.tmp1.Add(c.logSqrt10001, magicTickHigh), 128).Uint64())

	if tickLow == tickHigh {
		return tickLow, nil
	}

	// if err = c.GetSqrtRatioAtTickV2(int(tickHigh), c.sqrtRatio); err != nil {
	// 	return 0, err
	// }
	c.GetSqrtRatioAtTickV2(tickHigh, c.sqrtRatio)

	if c.sqrtRatio.Lt(sqrtRatioX96) {
		return tickHigh, nil
	}

	return tickLow, nil
}
