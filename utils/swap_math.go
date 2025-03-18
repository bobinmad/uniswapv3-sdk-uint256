package utils

import (
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/holiman/uint256"
)

const MaxFeeInt = 1000000

var MaxFeeUint256 = uint256.NewInt(MaxFeeInt)

type SwapStepCalculator struct {
	sqrtPriceCalculator SqrtPriceCalculator

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
		sqrtPriceCalculator: *NewSqrtPriceCalculator(),
		tmpUint256:          new(uint256.Int),
		amountRemainingU:    new(uint256.Int),
		maxFeeMinusFeePips:  new(uint256.Int),
		feePipsUin256Tmp:    new(uint256.Int),
		tmpUint256_2:        new(uint256.Int),
		tmpUint256_3:        new(uint256.Int),
		tmpUint256_4:        new(uint256.Int),
	}
}

func (c *SwapStepCalculator) ComputeSwapStep(
	sqrtRatioCurrentX96,
	sqrtRatioTargetX96 *Uint160,
	liquidity *Uint128,
	amountRemaining *Int256,
	feePips constants.FeeAmount,
	sqrtRatioNextX96 *Uint160, amountIn, amountOut, feeAmount *Uint256,
) error {
	zeroForOne := sqrtRatioCurrentX96.Cmp(sqrtRatioTargetX96) >= 0
	exactIn := amountRemaining.Sign() >= 0

	if exactIn {
		ToUInt256(amountRemaining, c.amountRemainingU)
	} else {
		ToUInt256(amountRemaining, c.amountRemainingU)
		c.amountRemainingU.Neg(c.amountRemainingU)
	}

	c.maxFeeMinusFeePips.SetUint64(MaxFeeInt - uint64(feePips))
	if exactIn {
		c.tmpUint256.Div(c.tmpUint256.Mul(c.amountRemainingU, c.maxFeeMinusFeePips), MaxFeeUint256)

		if zeroForOne {
			err := c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
			if err != nil {
				return err
			}
		} else {
			err := c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, true, amountIn)
			if err != nil {
				return err
			}
		}
		if c.tmpUint256.Cmp(amountIn) >= 0 {
			sqrtRatioNextX96.Set(sqrtRatioTargetX96)
		} else {
			err := c.sqrtPriceCalculator.GetNextSqrtPriceFromInput(sqrtRatioCurrentX96, liquidity, c.tmpUint256, zeroForOne, sqrtRatioNextX96)
			if err != nil {
				return err
			}
		}
	} else {
		if zeroForOne {
			err := c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioTargetX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
			if err != nil {
				return err
			}
		} else {
			err := c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioTargetX96, liquidity, false, amountOut)
			if err != nil {
				return err
			}
		}
		if c.amountRemainingU.Cmp(amountOut) >= 0 {
			sqrtRatioNextX96.Set(sqrtRatioTargetX96)
		} else {
			err := c.sqrtPriceCalculator.GetNextSqrtPriceFromOutput(sqrtRatioCurrentX96, liquidity, c.amountRemainingU, zeroForOne, sqrtRatioNextX96)
			if err != nil {
				return err
			}
		}
	}

	max := sqrtRatioTargetX96.Eq(sqrtRatioNextX96)

	if zeroForOne {
		if !(max && exactIn) {
			err := c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, true, amountIn)
			if err != nil {
				return err
			}
		}
		if !(max && !exactIn) {
			err := c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioNextX96, sqrtRatioCurrentX96, liquidity, false, amountOut)
			if err != nil {
				return err
			}
		}
	} else {
		if !(max && exactIn) {
			err := c.sqrtPriceCalculator.GetAmount1DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, true, amountIn)
			if err != nil {
				return err
			}
		}
		if !(max && !exactIn) {
			err := c.sqrtPriceCalculator.GetAmount0DeltaV2(sqrtRatioCurrentX96, sqrtRatioNextX96, liquidity, false, amountOut)
			if err != nil {
				return err
			}
		}
	}

	if !exactIn && amountOut.Gt(c.amountRemainingU) {
		amountOut.Set(c.amountRemainingU)
	}

	if exactIn && !sqrtRatioNextX96.Eq(sqrtRatioTargetX96) {
		// we didn't reach the target, so take the remainder of the maximum input as fee
		feeAmount.Sub(c.amountRemainingU, amountIn)
	} else {
		err := MulDivRoundingUpV2(amountIn, c.feePipsUin256Tmp.SetUint64(uint64(feePips)), c.maxFeeMinusFeePips, feeAmount)
		if err != nil {
			return err
		}
	}

	return nil
}
