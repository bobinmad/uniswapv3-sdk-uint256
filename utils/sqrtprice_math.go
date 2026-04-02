package utils

import (
	"errors"
	"math/big"

	"github.com/bobinmad/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
)

var (
	ErrSqrtPriceLessThanZero = errors.New("sqrt price less than zero")
	ErrLiquidityLessThanZero = errors.New("liquidity less than zero")
	ErrInvariant             = errors.New("invariant violation")
	ErrAddOverflow           = errors.New("add overflow")

	MaxUint160 = uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffff")
)

// func multiplyIn256(x, y, product *uint256.Int) *uint256.Int {
// 	return product.Mul(x, y) // no need to And with MaxUint256 here
// }

// func addIn256(x, y, sum *uint256.Int) *uint256.Int {
// 	return sum.Add(x, y) // no need to And with MaxUint256 here
// }

// deprecated
func GetAmount0Delta(sqrtRatioAX96, sqrtRatioBX96, liquidity *big.Int, roundUp bool) *big.Int {
	// panic("GetAmount0Delta() is deprecated")

	result := new(Uint256)
	NewSqrtPriceCalculator().GetAmount0DeltaV2(uint256.MustFromBig(sqrtRatioAX96), uint256.MustFromBig(sqrtRatioBX96), uint256.MustFromBig(liquidity), roundUp, result)

	return result.ToBig()
}

type SqrtPriceCalculator struct {
	fullMath                    *FullMath
	numerator1, numerator2, tmp *uint256.Int
	quotient                    *uint256.Int
	deno                        *uint256.Int
	product                     *uint256.Int
}

func NewSqrtPriceCalculator() *SqrtPriceCalculator {
	return &SqrtPriceCalculator{
		fullMath:   NewFullMath(),
		numerator1: new(uint256.Int),
		numerator2: new(uint256.Int),
		tmp:        new(uint256.Int),
		quotient:   new(uint256.Int),
		deno:       new(uint256.Int),
		product:    new(uint256.Int),
	}
}

func (c *SqrtPriceCalculator) GetAmount0DeltaV2(sqrtRatioAX96, sqrtRatioBX96 *Uint160, liquidity *Uint128, roundUp bool, result *Uint256) error {
	// https://github.com/Uniswap/v3-core/blob/d8b1c635c275d2a9450bd6a78f3fa2484fef73eb/contracts/libraries/SqrtPriceMath.sol#L159
	// if sqrtRatioAX96.Gt(sqrtRatioBX96) {
	// 	sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	// }

	c.numerator1[0] = 0
	c.numerator1[1] = liquidity[0] << 32
	c.numerator1[2] = liquidity[1]<<32 | liquidity[0]>>32
	c.numerator1[3] = liquidity[1] >> 32
	c.numerator2.Sub(sqrtRatioBX96, sqrtRatioAX96)

	if roundUp {
		if err := c.fullMath.MulDivRoundingUpV2(c.numerator1, c.numerator2, sqrtRatioBX96, c.deno); err != nil {
			return err
		}

		c.fullMath.DivRoundingUp(c.deno, sqrtRatioAX96, result)
		return nil
	}

	// : FullMath.mulDiv(numerator1, numerator2, sqrtRatioBX96) / sqrtRatioAX96;
	if err := c.fullMath.MulDivV2(c.numerator1, c.numerator2, sqrtRatioBX96, c.tmp, nil); err != nil {
		return err
	}

	c.fullMath.DivInto(c.tmp, sqrtRatioAX96, result)
	return nil
}

// deprecated
func GetAmount1Delta(sqrtRatioAX96, sqrtRatioBX96, liquidity *big.Int, roundUp bool) *big.Int {
	// panic("GetAmount1Delta() is deprecated")

	result := new(Uint256)
	NewSqrtPriceCalculator().GetAmount1DeltaV2(uint256.MustFromBig(sqrtRatioAX96), uint256.MustFromBig(sqrtRatioBX96), uint256.MustFromBig(liquidity), roundUp, result)
	return result.ToBig()
}

func (c *SqrtPriceCalculator) GetAmount1DeltaV2(sqrtRatioAX96, sqrtRatioBX96 *Uint160, liquidity *Uint128, roundUp bool, result *Uint256) error {
	// https://github.com/Uniswap/v3-core/blob/d8b1c635c275d2a9450bd6a78f3fa2484fef73eb/contracts/libraries/SqrtPriceMath.sol#L188
	// if sqrtRatioAX96.Gt(sqrtRatioBX96) {
	// 	sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	// }

	// Оптимизированный путь: denominator = Q96 = 2^96 (константа, степень двойки).
	// Вместо umul (16 ops) + Knuth-div (удалением): 6 ops Mul64 + сдвиг вправо на 96 бит.
	// Требования гарантированы типами: liquidity ≤ 2^128, diff ≤ 2^160.
	c.tmp.Sub(sqrtRatioBX96, sqrtRatioAX96)
	if roundUp {
		if mulRsh96_2x3(liquidity, c.tmp, result) {
			if result.Eq(MaxUint256) {
				return ErrInvariant
			}
			result.AddUint64(result, 1)
		}
		return nil
	}
	mulRsh96_2x3(liquidity, c.tmp, result)
	return nil
}

func (c *SqrtPriceCalculator) GetNextSqrtPriceFromInput(sqrtPX96 *Uint160, liquidity *Uint128, amountIn *uint256.Int, zeroForOne bool, result *Uint160) error {
	// if sqrtPX96.Sign() <= 0 || liquidity.Sign() <= 0 {
	// 	return ErrSqrtPriceLessThanZero
	// }

	if zeroForOne {
		return c.getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96, liquidity, amountIn, true, result)
	}

	return c.getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96, liquidity, amountIn, true, result)
}

func (c *SqrtPriceCalculator) GetNextSqrtPriceFromOutput(sqrtPX96 *Uint160, liquidity *Uint128, amountOut *uint256.Int, zeroForOne bool, result *Uint160) error {
	// if sqrtPX96.Sign() <= 0 || liquidity.Sign() <= 0 {
	// 	return ErrSqrtPriceLessThanZero
	// }

	if zeroForOne {
		return c.getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96, liquidity, amountOut, false, result)
	}

	return c.getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96, liquidity, amountOut, false, result)
}

func (c *SqrtPriceCalculator) getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96 *Uint160, liquidity *Uint128, amount *uint256.Int, add bool, result *Uint160) error {
	if amount.IsZero() {
		result.Set(sqrtPX96)
		return nil
	}

	// liquidity is always ≤ 128-bit: manual Lsh(96) = word-shift(1) + bit-shift(32),
	// avoids generic Lsh branches (liquidity[2]=liquidity[3]=0 guaranteed by Uint128).
	c.numerator1[0] = 0
	c.numerator1[1] = liquidity[0] << 32
	c.numerator1[2] = liquidity[1]<<32 | liquidity[0]>>32
	c.numerator1[3] = liquidity[1] >> 32

	c.product.Mul(amount, sqrtPX96)

	if add {
		c.fullMath.DivInto(c.product, amount, c.tmp)
		if c.tmp.Eq(sqrtPX96) {
			_, overflow := c.deno.AddOverflow(c.numerator1, c.product)
			if !overflow {
				return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.deno, result)
			}
		}

		c.fullMath.DivInto(c.numerator1, sqrtPX96, c.deno)
		c.deno.Add(c.deno, amount)
		c.fullMath.DivRoundingUp(c.numerator1, c.deno, result)
		return nil
	}

	c.fullMath.DivInto(c.product, amount, c.tmp)
	if !c.tmp.Eq(sqrtPX96) {
		return ErrInvariant
	}

	if !c.numerator1.Gt(c.product) {
		return ErrInvariant
	}

	return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.deno.Sub(c.numerator1, c.product), result)
}

func (c *SqrtPriceCalculator) getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96 *Uint160, liquidity *Uint128, amount *uint256.Int, add bool, result *Uint160) error {
	if add {
		// amount * Q96 == amount << 96 (mod 2^256) in both branches of the original code,
		// so the MaxUint160 check and branch are eliminated.
		// Manual Lsh(96) replaces Lsh/Mul wrappers; DivInto replaces holiman Div.
		c.tmp[0] = 0
		c.tmp[1] = amount[0] << 32
		c.tmp[2] = amount[1]<<32 | amount[0]>>32
		c.tmp[3] = amount[2]<<32 | amount[1]>>32
		c.fullMath.DivInto(c.tmp, liquidity, result)

		if _, overflow := result.AddOverflow(result, sqrtPX96); overflow {
			return ErrAddOverflow
		}
		return nil
	}

	if err := c.fullMath.MulDivRoundingUpV2(amount, constants.Q96U256, liquidity, result); err != nil {
		return err
	}

	if !sqrtPX96.Gt(result) {
		return ErrInvariant
	}

	result.Sub(sqrtPX96, result)
	return nil
}
