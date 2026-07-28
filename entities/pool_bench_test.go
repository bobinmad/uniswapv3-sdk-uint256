package entities

import (
	"testing"

	"github.com/bobinmad/uniswapv3-sdk-uint256/constants"
	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/vuquang23/int256"
)

var benchmarkPoolSwapTick int32

func newBaseWETHUSDCSwapBenchmarkPool() *Pool {
	const (
		firstTick   = int32(-205_000)
		lastTick    = int32(-195_000)
		currentTick = int32(-200_000)
		tickSpacing = int32(10)
	)

	liquidity := uint256.MustFromDecimal("987654321012345678")
	ticks := make([]Tick, 0, (lastTick-firstTick)/tickSpacing+1)
	for tick := firstTick; tick <= lastTick; tick += tickSpacing {
		ticks = append(ticks, Tick{
			Index:          tick,
			LiquidityGross: liquidity.Clone(),
			// Нулевой net оставляет active liquidity постоянной, но тик
			// остаётся initialized и проходит полный cross-tick hot path.
			LiquidityNet: new(int256.Int),
		})
	}

	tickHandler := NewTicksHandler()
	tickHandler.SetTicks(ticks)
	var sqrtPriceX96 utils.Uint160
	utils.NewTickCalculator().GetSqrtRatioAtTickV2(currentTick, &sqrtPriceX96)

	pool := NewPoolV3(
		common.Address{},
		uint16(constants.FeeLow),
		currentTick,
		&sqrtPriceX96,
		USDC,
		DAI,
		tickHandler,
	)
	pool.Liquidity.Set(liquidity)
	return pool
}

// BenchmarkPoolSwapBaseWETHUSDC измеряет основной интересующий симулятор hot
// path: Pool.Swap с 0.05% fee, callback вместо StepsFee и dense initialized
// ticks. Малый swap остаётся в одном bitmap word, большой пересекает тики.
func BenchmarkPoolSwapBaseWETHUSDC(b *testing.B) {
	for _, tc := range []struct {
		name     string
		amountIn *utils.Int256
		minCross int
	}{
		{
			name:     "single_step_callback",
			amountIn: int256.MustFromDec("100000000000000"),
			minCross: 0,
		},
		{
			name:     "cross_ticks_callback",
			amountIn: int256.MustFromDec("1000000000000000000000"),
			minCross: 1,
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			pool := newBaseWETHUSDCSwapBenchmarkPool()
			result := &SwapResultV2{
				FeeStepCallback: func(int32, *utils.Uint256, bool, *utils.Uint128) {},
			}
			if err := pool.Swap(true, tc.amountIn, nil, result); err != nil {
				b.Fatal(err)
			}
			if result.CrossInitTickLoops < tc.minCross {
				b.Fatalf("benchmark fixture crossed %d initialized ticks, want at least %d", result.CrossInitTickLoops, tc.minCross)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := pool.Swap(true, tc.amountIn, nil, result); err != nil {
					b.Fatal(err)
				}
			}
			benchmarkPoolSwapTick = result.CurrentTick
		})
	}
}
