package utils

import (
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
	feePipsUin256Tmp   *uint256.Int
	tmpUint256_2       *uint256.Int
	tmpUint256_3       *uint256.Int
	tmpUint256_4       *uint256.Int
}

func NewSwapStepCalculator() *SwapStepCalculator {
	return &SwapStepCalculator{
		sqrtPriceCalculator: NewSqrtPriceCalculator(),
		fullMath:            NewFullMath(),
		intTypes:            NewIntTypes(),

		tmpUint256:         new(uint256.Int),
		amountRemainingU:   new(uint256.Int),
		maxFeeMinusFeePips: new(uint256.Int),
		feePipsUin256Tmp:   new(uint256.Int),
		tmpUint256_2:       new(uint256.Int),
		tmpUint256_3:       new(uint256.Int),
		tmpUint256_4:       new(uint256.Int),
	}
}

func (c *SwapStepCalculator) ComputeSwapStep(
	sqrtRatioCurrentX96,
	sqrtRatioTargetX96 *Uint160,
	liquidity *Uint128,
	amountRemaining *Int256,
	feePips uint64,
	sqrtRatioNextX96 *Uint160, amountIn, amountOut, feeAmount *Uint256,
) error {
	var err error

	zeroForOne := !sqrtRatioCurrentX96.Lt(sqrtRatioTargetX96)
	exactIn := amountRemaining.Sign() >= 0

	c.maxFeeMinusFeePips.SetUint64(MaxFeeInt - feePips)
	if exactIn {
		c.intTypes.ToUInt256(amountRemaining, c.amountRemainingU)
		c.tmpUint256.Div(c.tmpUint256.Mul(c.amountRemainingU, c.maxFeeMinusFeePips), MaxFeeUint256)

		if zeroForOne {
			err = c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
		} else {
			err = c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, true, amountIn)
		}

		// >=
		if !c.tmpUint256.Lt(amountIn) {
			sqrtRatioNextX96.Set(sqrtRatioTargetX96)
		} else {
			err = c.sqrtPriceCalculator.GetNextSqrtPriceFromInput(sqrtRatioCurrentX96, liquidity, c.tmpUint256, zeroForOne, sqrtRatioNextX96)
		}
	} else {
		c.intTypes.ToUInt256(amountRemaining, c.amountRemainingU)
		c.amountRemainingU.Neg(c.amountRemainingU)

		if zeroForOne {
			err = c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
		} else {
			c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, false, amountOut)
		}

		if !c.amountRemainingU.Lt(amountOut) {
			sqrtRatioNextX96.Set(sqrtRatioTargetX96)
		} else {
			c.sqrtPriceCalculator.GetNextSqrtPriceFromOutput(sqrtRatioCurrentX96, liquidity, c.amountRemainingU, zeroForOne, sqrtRatioNextX96)
		}
	}
	// if err != nil {
	// 	return err
	// }

	max := sqrtRatioTargetX96.Eq(sqrtRatioNextX96)

	if zeroForOne {
		if !(max && exactIn) {
			err = c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
		}
		if !(max && !exactIn) {
			err = c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
		}
	} else {
		if !(max && exactIn) {
			err = c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, true, amountIn)

		}
		if !(max && !exactIn) {
			err = c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, false, amountOut)
		}
	}
	// if err != nil {
	// 	return err
	// }

	// if !exactIn && amountOut.Gt(c.amountRemainingU) {
	// 	amountOut.Set(c.amountRemainingU)
	// } else if exactIn && !sqrtRatioNextX96.Eq(sqrtRatioTargetX96) {
	// 	// we didn't reach the target, so take the remainder of the maximum input as fee
	// 	feeAmount.Sub(c.amountRemainingU, amountIn)
	// } else {
	// 	err = c.fullMath.MulDivRoundingUpV2(amountIn, c.feePipsUin256Tmp.SetUint64(uint64(feePips)), c.maxFeeMinusFeePips, feeAmount)
	// }

	if !exactIn && amountOut.Gt(c.amountRemainingU) {
		amountOut.Set(c.amountRemainingU)
	}

	if exactIn && !sqrtRatioNextX96.Eq(sqrtRatioTargetX96) {
		// we didn't reach the target, so take the remainder of the maximum input as fee
		feeAmount.Sub(c.amountRemainingU, amountIn)
	} else {
		err = c.fullMath.MulDivRoundingUpV2(amountIn, c.feePipsUin256Tmp.SetUint64(feePips), c.maxFeeMinusFeePips, feeAmount)
	}

	return err
}
