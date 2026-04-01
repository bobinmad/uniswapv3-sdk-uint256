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

	tmpUint256         *uint256.Int
	amountRemainingU   *uint256.Int
	maxFeeMinusFeePips *uint256.Int
	feePipsU256        *uint256.Int
	cachedFeePips      uint64
}

func NewSwapStepCalculator() *SwapStepCalculator {
	return &SwapStepCalculator{
		sqrtPriceCalculator: NewSqrtPriceCalculator(),
		fullMath:            NewFullMath(),
		intTypes:            NewIntTypes(),

		tmpUint256:         new(uint256.Int),
		amountRemainingU:   new(uint256.Int),
		maxFeeMinusFeePips: new(uint256.Int),
		feePipsU256:        new(uint256.Int),
		cachedFeePips:      math.MaxUint64, // sentinel: not yet initialized
	}
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
	if c.cachedFeePips != feePips {
		c.cachedFeePips = feePips
		c.maxFeeMinusFeePips.SetUint64(MaxFeeInt - feePips)
		c.feePipsU256.SetUint64(feePips)
	}

	if exactIn {
		c.amountRemainingU = (*uint256.Int)(amountRemaining)

		// Заменяем holiman.Div на divByMaxFeeInto: пропускает Gt-проверку и использует
		// предвычисленный реципрокал вместо hardware DIV в reciprocal2by1 (~30 цикл.).
		c.tmpUint256.Mul(c.amountRemainingU, c.maxFeeMinusFeePips)
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

		if zeroForOne {
			c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
		} else {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, false, amountOut)
		}

		if !c.amountRemainingU.Lt(amountOut) {
			*sqrtRatioNextX96 = *sqrtRatioTargetX96
		} else {
			c.sqrtPriceCalculator.GetNextSqrtPriceFromOutput(sqrtRatioCurrentX96, liquidity, c.amountRemainingU, zeroForOne, sqrtRatioNextX96)
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

	if !exactIn && amountOut.Gt(c.amountRemainingU) {
		*amountOut = *c.amountRemainingU
	}

	if exactIn && !sqrtRatioNextX96.Eq(sqrtRatioTargetX96) {
		// we didn't reach the target, so take the remainder of the maximum input as fee
		feeAmount.Sub(c.amountRemainingU, amountIn)
	} else {
		c.fullMath.MulDivRoundingUpV2(amountIn, c.feePipsU256, c.maxFeeMinusFeePips, feeAmount)
	}
}
