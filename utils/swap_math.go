package utils

import (
	"math"

	"github.com/holiman/uint256"
)

const MaxFeeInt = 1000000

var MaxFeeUint256 = uint256.NewInt(MaxFeeInt)

type SwapStepCalculator struct {
	sqrtPriceCalculator *SqrtPriceCalculator
	fullMath            *FullMath
	intTypes            *IntTypes

	tmpUint256             *uint256.Int
	amountRemainingU       *uint256.Int
	maxFeeMinusFeePips     *uint256.Int
	feeRatioNumerator      *uint256.Int
	feeRatioDenominator    *uint256.Int
	cachedFeePips          uint64
	feeRatioIsZero         bool
	feeRatioNumeratorIsOne bool
}

func NewSwapStepCalculator() *SwapStepCalculator {
	return &SwapStepCalculator{
		sqrtPriceCalculator: NewSqrtPriceCalculator(),
		fullMath:            NewFullMath(),
		intTypes:            NewIntTypes(),

		tmpUint256:          new(uint256.Int),
		amountRemainingU:    new(uint256.Int),
		maxFeeMinusFeePips:  new(uint256.Int),
		feeRatioNumerator:   new(uint256.Int),
		feeRatioDenominator: new(uint256.Int),
		cachedFeePips:       math.MaxUint64, // sentinel: not yet initialized
	}
}

func gcdUint64(a, b uint64) uint64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func (c *SwapStepCalculator) setFeePips(feePips uint64) {
	if c.cachedFeePips == feePips {
		return
	}

	c.cachedFeePips = feePips
	maxFeeMinusFeePips := uint64(MaxFeeInt) - feePips
	c.maxFeeMinusFeePips.SetUint64(maxFeeMinusFeePips)

	gcd := gcdUint64(feePips, maxFeeMinusFeePips)
	numerator := feePips / gcd
	denominator := maxFeeMinusFeePips / gcd
	c.feeRatioNumerator.SetUint64(numerator)
	c.feeRatioDenominator.SetUint64(denominator)
	c.feeRatioIsZero = numerator == 0
	c.feeRatioNumeratorIsOne = numerator == 1
}

func (c *SwapStepCalculator) computeFeeAmount(amountIn, feeAmount *uint256.Int) {
	if c.feeRatioIsZero {
		feeAmount.Clear()
		return
	}
	if c.feeRatioNumeratorIsOne {
		c.fullMath.DivRoundingUp(amountIn, c.feeRatioDenominator, feeAmount)
		return
	}
	c.fullMath.MulDivRoundingUpV2(amountIn, c.feeRatioNumerator, c.feeRatioDenominator, feeAmount)
}

func (c *SwapStepCalculator) ComputeSwapStep(
	sqrtRatioCurrentX96,
	sqrtRatioTargetX96 *Uint160,
	liquidity *Uint128,
	amountRemaining *Int256,
	feePips uint64,
	sqrtRatioNextX96 *Uint160, amountIn, amountOut, feeAmount *Uint256,
	zeroForOne, exactIn bool,
) {
	// cache fee constants: typically constant across all steps of one swap
	c.setFeePips(feePips)

	// В exact-input можно читать amountRemaining zero-copy, но нельзя сохранять этот
	// указатель в scratch-поле calculator-а. Pool переиспользует один и тот же signed
	// amountSpecifiedRemaining между Swap-вызовами; после exact-input сохранённый alias
	// заставлял следующий exact-output делать Neg in-place и портить signed remainder.
	amountRemainingU := c.amountRemainingU
	if exactIn {
		amountRemainingU = (*uint256.Int)(amountRemaining)

		// Заменяем holiman.Div на divByMaxFeeInto: пропускает Gt-проверку и использует
		// предвычисленный реципрокал вместо hardware DIV в reciprocal2by1 (~30 цикл.).
		c.tmpUint256.Mul(amountRemainingU, c.maxFeeMinusFeePips)
		divByMaxFeeInto(c.tmpUint256, c.tmpUint256)

		if zeroForOne {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
		} else {
			c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, true, amountIn)
		}

		// >=
		if !c.tmpUint256.Lt(amountIn) {
			*sqrtRatioNextX96 = *sqrtRatioTargetX96
		} else {
			c.sqrtPriceCalculator.GetNextSqrtPriceFromInput(sqrtRatioCurrentX96, liquidity, c.tmpUint256, zeroForOne, sqrtRatioNextX96)
		}
	} else {
		c.amountRemainingU.Neg((*uint256.Int)(amountRemaining))
		amountRemainingU = c.amountRemainingU

		if zeroForOne {
			c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
		} else {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, false, amountOut)
		}

		if !amountRemainingU.Lt(amountOut) {
			*sqrtRatioNextX96 = *sqrtRatioTargetX96
		} else {
			c.sqrtPriceCalculator.GetNextSqrtPriceFromOutput(sqrtRatioCurrentX96, liquidity, amountRemainingU, zeroForOne, sqrtRatioNextX96)
		}
	}

	max := sqrtRatioTargetX96.Eq(sqrtRatioNextX96)

	if zeroForOne {
		if !(max && exactIn) {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
		}
		if !(max && !exactIn) {
			c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
		}
	} else {
		if !(max && exactIn) {
			c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, true, amountIn)
		}
		if !(max && !exactIn) {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, false, amountOut)
		}
	}

	if !exactIn && amountOut.Gt(amountRemainingU) {
		*amountOut = *amountRemainingU
	}

	if exactIn && !sqrtRatioNextX96.Eq(sqrtRatioTargetX96) {
		// we didn't reach the target, so take the remainder of the maximum input as fee
		feeAmount.Sub(amountRemainingU, amountIn)
	} else {
		// Сокращаем feePips/(MaxFee-feePips) при смене fee tier. Для
		// 0.01%, 0.05% и 1% числитель становится 1, поэтому общий 512-bit
		// MulDiv заменяется на одно DivRoundingUp.
		c.computeFeeAmount(amountIn, feeAmount)
	}
}
