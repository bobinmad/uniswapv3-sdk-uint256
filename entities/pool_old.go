package entities

import (
	"errors"
	"fmt"

	"github.com/KyberNetwork/int256"
	kyberEntities "github.com/KyberNetwork/uniswapv3-sdk-uint256/entities"
	"github.com/holiman/uint256"

	kyberconstants "github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	kyberutils "github.com/KyberNetwork/uniswapv3-sdk-uint256/utils"
	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
)

// SwapOld - прокси на старую версию метода Swap из оригинальной библиотеки KyberSwap
// Использует оригинальные функции из kyberutils вместо новых калькуляторов
func (p *Pool) SwapOld(zeroForOne bool, amountSpecified *utils.Int256, sqrtPriceLimitX96 *utils.Uint160, swapResult *SwapResultV2) error {
	var err error

	if sqrtPriceLimitX96 == nil {
		if zeroForOne {
			sqrtPriceLimitX96 = new(uint256.Int).AddUint64(kyberutils.MinSqrtRatioU256, 1)
		} else {
			sqrtPriceLimitX96 = new(uint256.Int).SubUint64(kyberutils.MaxSqrtRatioU256, 1)
		}
	}

	if zeroForOne {
		// zeroForOne: цена должна убывать и быть > Min, < текущей
		if sqrtPriceLimitX96.Cmp(kyberutils.MinSqrtRatioU256) <= 0 {
			sqrtPriceLimitX96 = new(uint256.Int).AddUint64(kyberutils.MinSqrtRatioU256, 1)
		}
		if sqrtPriceLimitX96.Cmp(p.SqrtRatioX96) >= 0 {
			sqrtPriceLimitX96 = new(uint256.Int).SubUint64(p.SqrtRatioX96, 1)
		}
	} else {
		// oneForZero: цена должна возрастать и быть < Max, > текущей
		if sqrtPriceLimitX96.Cmp(kyberutils.MaxSqrtRatioU256) >= 0 {
			sqrtPriceLimitX96 = new(uint256.Int).SubUint64(kyberutils.MaxSqrtRatioU256, 1)
		}
		if sqrtPriceLimitX96.Cmp(p.SqrtRatioX96) <= 0 {
			sqrtPriceLimitX96 = new(uint256.Int).AddUint64(p.SqrtRatioX96, 1)
		}
	}

	exactInput := amountSpecified.Sign() >= 0

	// keep track of swap state (как в оригинальной версии)
	state := struct {
		amountSpecifiedRemaining *utils.Int256
		amountCalculated         *utils.Int256
		sqrtPriceX96             *utils.Uint160
		tick                     int // В оригинале int, не int32!
		liquidity                *utils.Uint128
	}{
		amountSpecifiedRemaining: new(utils.Int256).Set(amountSpecified),
		amountCalculated:         int256.NewInt(0),
		sqrtPriceX96:             new(utils.Uint160).Set(p.SqrtRatioX96),
		tick:                     int(p.TickCurrent), // конвертация int32 -> int
		liquidity:                new(utils.Uint128).Set(p.Liquidity),
	}

	if swapResult.StepsFee == nil {
		swapResult.StepsFee = make([]StepFeeResult, 0, 16)
	} else {
		swapResult.StepsFee = swapResult.StepsFee[:0]
	}
	swapResult.CrossInitTickLoops = 0

	// start swap while loop
	fmt.Printf("[DEBUG SWAP] Starting swap: amountSpecified=%s, exactInput=%v, liquidity=%s\n", amountSpecified.Dec(), exactInput, state.liquidity)

	for !state.amountSpecifiedRemaining.IsZero() && state.sqrtPriceX96.Cmp(sqrtPriceLimitX96) != 0 {
		var step StepComputations
		step.sqrtPriceStartX96.Set(state.sqrtPriceX96)

		// NextInitializedTickIndex возвращает int32
		var tickNext32 int32
		tickNext32, step.initialized, err = p.TickDataProvider.NextInitializedTickIndex(int32(state.tick), zeroForOne)

		fmt.Printf("[DEBUG] state.tick=%d, liquidity=%s, amountRemaining=%s, tickNext32=%d, initialized=%v\n", state.tick, state.liquidity, state.amountSpecifiedRemaining.Dec(), tickNext32, step.initialized)

		if err != nil {
			// Обработка случая, когда больше нет инициализированных тиков
			// В этом случае продолжаем swap до граничного тика
			fmt.Printf("[DEBUG] NextInitializedTickIndex error: %v, tick=%d, zeroForOne=%v\n", err, state.tick, zeroForOne)
			if errors.Is(err, kyberEntities.ErrBelowSmallest) {
				fmt.Printf("[DEBUG] Handling ErrBelowSmallest, setting tickNext to MinTick\n")
				tickNext32 = utils.MinTick
				step.initialized = false
			} else if errors.Is(err, kyberEntities.ErrAtOrAboveLargest) {
				fmt.Printf("[DEBUG] Handling ErrAtOrAboveLargest, setting tickNext to MaxTick\n")
				tickNext32 = utils.MaxTick
				step.initialized = false
			} else {
				return err
			}
		}
		step.tickNext = int32(tickNext32) // В нашей структуре tickNext это int32

		tickNext := int(tickNext32) // для работы со старым API

		if tickNext < kyberutils.MinTick {
			tickNext = kyberutils.MinTick
		} else if tickNext > kyberutils.MaxTick {
			tickNext = kyberutils.MaxTick
		}

		// Используем оригинальную функцию из KyberNetwork
		err = kyberutils.GetSqrtRatioAtTickV2(tickNext, &step.sqrtPriceNextX96)
		if err != nil {
			return err
		}

		var targetValue utils.Uint160
		if zeroForOne {
			if step.sqrtPriceNextX96.Cmp(sqrtPriceLimitX96) < 0 {
				targetValue.Set(sqrtPriceLimitX96)
			} else {
				targetValue.Set(&step.sqrtPriceNextX96)
			}
		} else {
			if step.sqrtPriceNextX96.Cmp(sqrtPriceLimitX96) > 0 {
				targetValue.Set(sqrtPriceLimitX96)
			} else {
				targetValue.Set(&step.sqrtPriceNextX96)
			}
		}

		var nxtSqrtPriceX96 utils.Uint160
		// Используем оригинальную функцию ComputeSwapStep из KyberNetwork
		// Конвертируем FeeAmount между пакетами (оба uint64)
		kyberFee := kyberconstants.FeeAmount(p.Fee)
		err = kyberutils.ComputeSwapStep(
			state.sqrtPriceX96,
			&targetValue,
			state.liquidity,
			state.amountSpecifiedRemaining,
			kyberFee,
			&nxtSqrtPriceX96,
			&step.amountIn,
			&step.amountOut,
			&step.feeAmount,
		)
		if err != nil {
			return err
		}
		state.sqrtPriceX96.Set(&nxtSqrtPriceX96)

		var amountInPlusFee utils.Uint256
		amountInPlusFee.Add(&step.amountIn, &step.feeAmount)

		var amountInPlusFeeSigned utils.Int256
		// Используем оригинальную функцию ToInt256 из KyberNetwork
		err = kyberutils.ToInt256(&amountInPlusFee, &amountInPlusFeeSigned)
		if err != nil {
			return err
		}

		var amountOutSigned utils.Int256
		err = kyberutils.ToInt256(&step.amountOut, &amountOutSigned)
		if err != nil {
			return err
		}

		if exactInput {
			state.amountSpecifiedRemaining.Sub(state.amountSpecifiedRemaining, &amountInPlusFeeSigned)
			state.amountCalculated.Sub(state.amountCalculated, &amountOutSigned)
		} else {
			state.amountSpecifiedRemaining.Add(state.amountSpecifiedRemaining, &amountOutSigned)
			state.amountCalculated.Add(state.amountCalculated, &amountInPlusFeeSigned)
		}

		swapResult.StepsFee = append(swapResult.StepsFee, StepFeeResult{
			Tick:       int32(state.tick),
			FeeAmount:  step.feeAmount,
			ZeroForOne: zeroForOne,
			Liquidity:  *state.liquidity,
		})

		// Guard: при нулевой ликвидности не двигаем цену дальше, если мы не на границе init-tick
		if state.liquidity.IsZero() && state.sqrtPriceX96.Cmp(&step.sqrtPriceNextX96) != 0 {
			break
		}

		if state.sqrtPriceX96.Cmp(&step.sqrtPriceNextX96) == 0 {
			// if the tick is initialized, run the tick transition
			if step.initialized {
				tick, err := p.TickDataProvider.GetTick(int32(tickNext))
				if err != nil {
					return err
				}

				liquidityNet := tick.LiquidityNet
				fmt.Printf("[DEBUG TICK CROSS] Crossing tick %d, liquidityBefore=%s, liquidityNet=%s, zeroForOne=%v\n", tickNext, state.liquidity, liquidityNet.Dec(), zeroForOne)

				// if we're moving leftward, we interpret liquidityNet as the opposite sign
				if zeroForOne {
					liquidityNet = new(utils.Int128).Neg(liquidityNet)
				}
				// Используем оригинальную функцию AddDeltaInPlace из KyberNetwork
				kyberutils.AddDeltaInPlace(state.liquidity, liquidityNet)

				fmt.Printf("[DEBUG TICK CROSS] After crossing tick %d, liquidityAfter=%s\n", tickNext, state.liquidity)

				swapResult.CrossInitTickLoops++
			}

			if zeroForOne {
				state.tick = tickNext - 1
			} else {
				state.tick = tickNext
			}

		} else if state.sqrtPriceX96.Cmp(&step.sqrtPriceStartX96) != 0 {
			// recompute unless we're on a lower tick boundary
			// Используем оригинальную функцию GetTickAtSqrtRatioV2 из KyberNetwork
			state.tick, err = kyberutils.GetTickAtSqrtRatioV2(state.sqrtPriceX96)
			if err != nil {
				return err
			}
		}

		if swapResult.CrossInitTickLoops > MAX_CROSS_INIT_TICK_LOOPS {
			return fmt.Errorf("max cross init tick loops %d reached", MAX_CROSS_INIT_TICK_LOOPS)
		}
	}

	swapResult.AmountCalculated = state.amountCalculated
	swapResult.SqrtRatioX96 = state.sqrtPriceX96
	swapResult.Liquidity = state.liquidity
	swapResult.CurrentTick = int32(state.tick) // конвертация int -> int32
	swapResult.RemainingAmountIn = state.amountSpecifiedRemaining

	return nil
}
