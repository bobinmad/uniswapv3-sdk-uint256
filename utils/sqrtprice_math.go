package utils

import (
	"errors"
	"math/big"

	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
)

var (
	ErrSqrtPriceLessThanZero = errors.New("sqrt price less than zero")
	ErrLiquidityLessThanZero = errors.New("liquidity less than zero")
	ErrInvariant             = errors.New("invariant violation")
	ErrAddOverflow           = errors.New("add overflow")

	MaxUint160 = uint256.MustFromHex("0xffffffffffffffffffffffffffffffffffffffff")
)

func multiplyIn256(x, y, product *uint256.Int) *uint256.Int {
	return product.Mul(x, y) // no need to And with MaxUint256 here
}

func addIn256(x, y, sum *uint256.Int) *uint256.Int {
	return sum.Add(x, y) // no need to And with MaxUint256 here
}

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
	diff                        *uint256.Int
	deno                        *uint256.Int
	product                     *uint256.Int
	denominator                 *uint256.Int
}

func NewSqrtPriceCalculator() *SqrtPriceCalculator {
	return &SqrtPriceCalculator{
		fullMath:    NewFullMath(),
		numerator1:  new(uint256.Int),
		numerator2:  new(uint256.Int),
		tmp:         new(uint256.Int),
		quotient:    new(uint256.Int),
		diff:        new(uint256.Int),
		deno:        new(uint256.Int),
		product:     new(uint256.Int),
		denominator: new(uint256.Int),
	}
}

func (c *SqrtPriceCalculator) GetAmount0DeltaV2(sqrtRatioAX96, sqrtRatioBX96 *Uint160, liquidity *Uint128, roundUp bool, result *Uint256) error {
	// https://github.com/Uniswap/v3-core/blob/d8b1c635c275d2a9450bd6a78f3fa2484fef73eb/contracts/libraries/SqrtPriceMath.sol#L159
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	c.numerator1.Lsh(new(uint256.Int).Set(liquidity), 96)
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

	result.Div(c.tmp, sqrtRatioAX96)
	return nil
}

// deprecated
func GetAmount1Delta(sqrtRatioAX96, sqrtRatioBX96, liquidity *big.Int, roundUp bool) *big.Int {
	// panic("GetAmount1Delta() is deprecated")

	result := new(Uint256)
	NewSqrtPriceCalculator().GetAmount0DeltaV2(uint256.MustFromBig(sqrtRatioAX96), uint256.MustFromBig(sqrtRatioBX96), uint256.MustFromBig(liquidity), roundUp, result)
	return result.ToBig()
}

func (c *SqrtPriceCalculator) GetAmount1DeltaV2(sqrtRatioAX96, sqrtRatioBX96 *Uint160, liquidity *Uint128, roundUp bool, result *Uint256) error {
	var err error

	// https://github.com/Uniswap/v3-core/blob/d8b1c635c275d2a9450bd6a78f3fa2484fef73eb/contracts/libraries/SqrtPriceMath.sol#L188
	if sqrtRatioAX96.Gt(sqrtRatioBX96) {
		sqrtRatioAX96, sqrtRatioBX96 = sqrtRatioBX96, sqrtRatioAX96
	}

	c.diff.Sub(sqrtRatioBX96, sqrtRatioAX96)
	if roundUp {
		if err = c.fullMath.MulDivRoundingUpV2(liquidity, c.diff, constants.Q96U256, result); err != nil {
			return err
		}

		return nil
	}

	// : FullMath.mulDiv(liquidity, sqrtRatioBX96 - sqrtRatioAX96, FixedPoint96.Q96);
	if err = c.fullMath.MulDivV2(liquidity, c.diff, constants.Q96U256, result, nil); err != nil {
		return err
	}
	return nil
}

func (c *SqrtPriceCalculator) GetNextSqrtPriceFromInput(sqrtPX96 *Uint160, liquidity *Uint128, amountIn *uint256.Int, zeroForOne bool, result *Uint160) error {
	if sqrtPX96.Sign() <= 0 {
		return ErrSqrtPriceLessThanZero
	}
	if liquidity.Sign() <= 0 {
		return ErrLiquidityLessThanZero
	}
	if zeroForOne {
		return c.getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96, liquidity, amountIn, true, result)
	}
	return c.getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96, liquidity, amountIn, true, result)
}

func (c *SqrtPriceCalculator) GetNextSqrtPriceFromOutput(sqrtPX96 *Uint160, liquidity *Uint128, amountOut *uint256.Int, zeroForOne bool, result *Uint160) error {
	if sqrtPX96.Sign() <= 0 {
		return ErrSqrtPriceLessThanZero
	}
	if liquidity.Sign() <= 0 {
		return ErrLiquidityLessThanZero
	}
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

	c.numerator1.Lsh(liquidity, 96)
	multiplyIn256(amount, sqrtPX96, c.product)

	if add {
		if c.tmp.Div(c.product, amount).Eq(sqrtPX96) {
			addIn256(c.numerator1, c.product, c.denominator)
			// >=
			if !c.denominator.Lt(c.numerator1) {
				return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.denominator, result)
			}
		}

		c.fullMath.DivRoundingUp(c.numerator1, c.tmp.Add(c.tmp.Div(c.numerator1, sqrtPX96), amount), result)

		return nil
	} else {
		if !c.tmp.Div(c.product, amount).Eq(sqrtPX96) {
			return ErrInvariant
		}
		// if c.numerator1.Cmp(c.product) <= 0 {
		if !c.numerator1.Gt(c.product) {
			return ErrInvariant
		}

		return c.fullMath.MulDivRoundingUpV2(c.numerator1, sqrtPX96, c.denominator.Sub(c.numerator1, c.product), result)
	}
}

func (c *SqrtPriceCalculator) getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96 *Uint160, liquidity *Uint128, amount *uint256.Int, add bool, result *Uint160) error {
	var err error

	if add {
		// <=
		// if amount.Cmp(MaxUint160) <= 0 {
		if !amount.Gt(MaxUint160) {
			c.quotient.Div(c.tmp.Lsh(amount, 96), liquidity)
		} else {
			c.quotient.Div(c.tmp.Mul(amount, constants.Q96U256), liquidity)
		}

		if _, overflow := c.quotient.AddOverflow(c.quotient, sqrtPX96); overflow {
			return ErrAddOverflow
		}

		if err = CheckToUint160(c.quotient); err != nil {
			return err
		}

		result.Set(c.quotient)
		return nil
	}

	if err = c.fullMath.MulDivRoundingUpV2(amount, constants.Q96U256, liquidity, c.quotient); err != nil {
		return err
	}

	// <=
	// if sqrtPX96.Cmp(c.quotient) <= 0 {
	if !sqrtPX96.Gt(c.quotient) {
		return ErrInvariant
	}

	// always fits 160 bits
	result.Set(c.quotient.Sub(sqrtPX96, c.quotient))
	return nil
}
