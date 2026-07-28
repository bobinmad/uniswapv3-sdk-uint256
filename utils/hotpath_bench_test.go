package utils

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/vuquang23/int256"
)

var (
	benchmarkUint256Result Uint256
	benchmarkSwapStepTick  int32
)

// BenchmarkFullMathDivIntoFeeReward фиксирует профиль деления из начисления
// Uniswap fee в defisimulator: (ourLiquidity * feeAmount) / poolLiquidity.
// В реальных пулах liquidity бывает как одно-, так и двухсловной.
func BenchmarkFullMathDivIntoFeeReward(b *testing.B) {
	numerator := uint256.MustFromDecimal("6135792468123456789012345")

	for _, tc := range []struct {
		name        string
		denominator *uint256.Int
	}{
		{name: "one_word_liquidity", denominator: uint256.MustFromDecimal("987654321012345678")},
		{name: "two_word_liquidity", denominator: uint256.MustFromDecimal("98765432101234567890")},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fullMath := NewFullMath()
			var result Uint256
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				fullMath.DivInto(numerator, tc.denominator, &result)
			}
			benchmarkUint256Result = result
		})
	}
}

// BenchmarkComputeSwapStepBaseWETHUSDC фиксирует форму ComputeSwapStep из
// Base WETH/USDC 0.05%: tick около -200k, spacing=10 и exact-input swaps в
// обе стороны. Оба случая достигают следующего tick boundary и включают
// расчёт fee через MulDivRoundingUpV2.
func BenchmarkComputeSwapStepBaseWETHUSDC(b *testing.B) {
	tickCalculator := NewTickCalculator()
	var current, downTarget, upTarget Uint160
	tickCalculator.GetSqrtRatioAtTickV2(-200_000, &current)
	tickCalculator.GetSqrtRatioAtTickV2(-200_010, &downTarget)
	tickCalculator.GetSqrtRatioAtTickV2(-199_990, &upTarget)

	liquidity := uint256.MustFromDecimal("987654321012345678")
	cases := []struct {
		name       string
		target     *Uint160
		amount     *Int256
		zeroForOne bool
	}{
		{
			name:       "zero_for_one",
			target:     &downTarget,
			amount:     int256.MustFromDec("1000000000000000000"),
			zeroForOne: true,
		},
		{
			name:       "one_for_zero",
			target:     &upTarget,
			amount:     int256.MustFromDec("2000000000"),
			zeroForOne: false,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			calculator := NewSwapStepCalculator()
			var sqrtRatioNextX96 Uint160
			var amountIn, amountOut, feeAmount Uint256
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				calculator.ComputeSwapStep(
					&current,
					tc.target,
					liquidity,
					tc.amount,
					500,
					&sqrtRatioNextX96,
					&amountIn,
					&amountOut,
					&feeAmount,
					tc.zeroForOne,
					true,
				)
			}
			benchmarkUint256Result = feeAmount
			benchmarkSwapStepTick = int32(sqrtRatioNextX96[0])
		})
	}
}
