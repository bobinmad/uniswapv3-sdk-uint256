package entities

import (
	"errors"
	"math/big"

	"github.com/KyberNetwork/int256"
	"github.com/daoleno/uniswap-sdk-core/entities"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/KyberNetwork/uniswapv3-sdk-uint256/constants"
	"github.com/KyberNetwork/uniswapv3-sdk-uint256/utils"
)

var (
	ErrFeeTooHigh               = errors.New("fee too high")
	ErrInvalidSqrtRatioX96      = errors.New("invalid sqrtRatioX96")
	ErrTokenNotInvolved         = errors.New("token not involved in pool")
	ErrSqrtPriceLimitX96TooLow  = errors.New("SqrtPriceLimitX96 too low")
	ErrSqrtPriceLimitX96TooHigh = errors.New("SqrtPriceLimitX96 too high")
)

type StepComputations struct {
	sqrtPriceStartX96 utils.Uint160
	tickNext          int
	initialized       bool
	sqrtPriceNextX96  utils.Uint160
	amountIn          utils.Uint256
	amountOut         utils.Uint256
	feeAmount         utils.Uint256
}

// Represents a V3 pool
type Pool struct {
	Token0           *entities.Token
	Token1           *entities.Token
	Fee              constants.FeeAmount
	SqrtRatioX96     *utils.Uint160
	Liquidity        *utils.Uint128
	TickCurrent      int
	TickDataProvider TickDataProvider
	tickCalculator   *utils.TickCalculator
	intTypes         *utils.IntTypes

	token0Price *entities.Price
	token1Price *entities.Price

	// tmp vars
	lastState             State
	liquidityNet          *utils.Int128
	amountInPlusFee       *utils.Uint256
	amountInPlusFeeSigned *utils.Int256
	nxtSqrtPriceX96       *utils.Uint160
	targetValue           *utils.Uint160
	amountOutSigned       *utils.Int256
	step                  StepComputations
	swapStepCalculator    *utils.SwapStepCalculator

	tmpUint256   *uint256.Int
	tmpUint256_1 *uint256.Int
	tmpUint256_2 *uint256.Int
	tmpUint256_3 *uint256.Int
	tmpUint256_4 *uint256.Int
	tmpUint256_5 *uint256.Int
	tmpUint256_6 *uint256.Int
}

type SwapResult struct {
	// amountCalculated   *utils.Int256
	// sqrtRatioX96       *utils.Uint160
	// liquidity          *utils.Uint128
	// remainingAmountIn  *utils.Int256
	// currentTick        int
	// crossInitTickLoops int
}

type GetAmountResult struct {
	ReturnedAmount     *entities.CurrencyAmount
	RemainingAmountIn  *entities.CurrencyAmount
	NewPoolState       *Pool
	CrossInitTickLoops int
}

type GetAmountResultV2 struct {
	ReturnedAmount     *utils.Int256
	RemainingAmountIn  *utils.Int256
	SqrtRatioX96       *utils.Uint160
	Liquidity          *utils.Uint128
	CurrentTick        int
	CrossInitTickLoops int
}

func GetAddress(tokenA, tokenB *entities.Token, fee constants.FeeAmount,
	initCodeHashManualOverride string) (common.Address, error) {
	return utils.ComputePoolAddress(constants.FactoryAddress, tokenA, tokenB, fee, initCodeHashManualOverride)
}

// deprecated
func NewPool(tokenA, tokenB *entities.Token, fee constants.FeeAmount, sqrtRatioX96 *big.Int, liquidity *big.Int,
	tickCurrent int, ticks TickDataProvider) (*Pool, error) {
	return NewPoolV2(
		tokenA, tokenB, fee,
		uint256.MustFromBig(sqrtRatioX96),
		uint256.MustFromBig(liquidity),
		tickCurrent,
		ticks,
	)
}

/**
 * Construct a pool
 * @param tokenA One of the tokens in the pool
 * @param tokenB The other token in the pool
 * @param fee The fee in hundredths of a bips of the input amount of every swap that is collected by the pool
 * @param sqrtRatioX96 The sqrt of the current ratio of amounts of token1 to token0
 * @param liquidity The current value of in range liquidity
 * @param tickCurrent The current tick of the pool
 * @param ticks The current state of the pool ticks or a data provider that can return tick data
 */
func NewPoolV2(tokenA, tokenB *entities.Token, fee constants.FeeAmount, sqrtRatioX96 *utils.Uint160,
	liquidity *utils.Uint128, tickCurrent int, ticks TickDataProvider) (*Pool, error) {
	if fee >= constants.FeeMax {
		return nil, ErrFeeTooHigh
	}

	tickCalculator := utils.NewTickCalculator()

	var tickCurrentSqrtRatioX96, nextTickSqrtRatioX96 utils.Uint160
	tickCalculator.GetSqrtRatioAtTickV2(tickCurrent, &tickCurrentSqrtRatioX96)
	tickCalculator.GetSqrtRatioAtTickV2(tickCurrent+1, &nextTickSqrtRatioX96)

	if sqrtRatioX96.Lt(&tickCurrentSqrtRatioX96) || sqrtRatioX96.Gt(&nextTickSqrtRatioX96) {
		return nil, ErrInvalidSqrtRatioX96
	}
	token0 := tokenA
	token1 := tokenB
	isSorted, err := utils.SortsBefore(tokenA, tokenB)
	if err != nil {
		return nil, err
	}
	if !isSorted {
		token0 = tokenB
		token1 = tokenA
	}

	return &Pool{
		Token0:           token0,
		Token1:           token1,
		Fee:              fee,
		SqrtRatioX96:     sqrtRatioX96,
		Liquidity:        liquidity,
		TickCurrent:      tickCurrent,
		TickDataProvider: ticks,
	}, nil
}

func NewPoolV3(
	fee uint16,
	initTick int,
	initSqrtPriceX96 *utils.Uint160,
	token0, token1 *entities.Token,
	ticksHandler TickDataProvider,
) *Pool {
	return &Pool{
		Fee:              constants.FeeAmount(fee),
		TickDataProvider: ticksHandler,
		TickCurrent:      initTick,
		SqrtRatioX96:     initSqrtPriceX96.Clone(),
		Liquidity:        new(utils.Uint128),
		Token0:           token0,
		Token1:           token1,
		lastState: State{
			amountSpecifiedRemaining: new(utils.Int256),
			amountCalculated:         new(utils.Int256),
			sqrtPriceX96:             new(utils.Uint160),
			liquidity:                new(utils.Uint128),
		},
		liquidityNet:       new(utils.Int128),
		swapStepCalculator: utils.NewSwapStepCalculator(),
		tickCalculator:     utils.NewTickCalculator(),
		intTypes:           utils.NewIntTypes(),

		amountInPlusFee:       new(utils.Uint256),
		amountInPlusFeeSigned: new(utils.Int256),
		nxtSqrtPriceX96:       new(utils.Uint160),
		targetValue:           new(utils.Uint160),
		amountOutSigned:       new(utils.Int256),

		tmpUint256:   new(uint256.Int),
		tmpUint256_1: new(uint256.Int),
		tmpUint256_2: new(uint256.Int),
		tmpUint256_3: new(uint256.Int),
		tmpUint256_4: new(uint256.Int),
		tmpUint256_5: new(uint256.Int),
		tmpUint256_6: new(uint256.Int),
	}
}

/**
 * Returns true if the token is either token0 or token1
 * @param token The token to check
 * @returns True if token is either token0 or token
 */
func (p *Pool) InvolvesToken(token *entities.Token) bool {
	return p.Token0.Equal(token) || p.Token1.Equal(token)
}

// Token0Price returns the current mid price of the pool in terms of token0, i.e. the ratio of token1 over token0
func (p *Pool) Token0Price() *entities.Price {
	if p.token0Price != nil {
		return p.token0Price
	}
	p.token0Price = entities.NewPrice(p.Token0, p.Token1, constants.Q192,
		new(uint256.Int).Mul(p.SqrtRatioX96, p.SqrtRatioX96).ToBig())
	return p.token0Price
}

// Token1Price returns the current mid price of the pool in terms of token1, i.e. the ratio of token0 over token1
func (p *Pool) Token1Price() *entities.Price {
	if p.token1Price != nil {
		return p.token1Price
	}
	p.token1Price = entities.NewPrice(p.Token1, p.Token0, new(uint256.Int).Mul(p.SqrtRatioX96, p.SqrtRatioX96).ToBig(),
		constants.Q192)
	return p.token1Price
}

/**
 * Return the price of the given token in terms of the other token in the pool.
 * @param token The token to return price of
 * @returns The price of the given token, in terms of the other.
 */
func (p *Pool) PriceOf(token *entities.Token) (*entities.Price, error) {
	if !p.InvolvesToken(token) {
		return nil, ErrTokenNotInvolved
	}
	if p.Token0.Equal(token) {
		return p.Token0Price(), nil
	}
	return p.Token1Price(), nil
}

// ChainId returns the chain ID of the tokens in the pool.
func (p *Pool) ChainID() uint {
	return p.Token0.ChainId()
}

/**
 * Given an input amount of a token, return the computed output amount, and a pool with state updated after the trade
 * @param inputAmount The input amount for which to quote the output amount
 * @param sqrtPriceLimitX96 The Q64.96 sqrt price limit
 * @returns The output amount and the pool with updated state
 */
func (p *Pool) GetOutputAmount(inputAmount *entities.CurrencyAmount,
	sqrtPriceLimitX96 *utils.Uint160) (*GetAmountResult, error) {
	if !(inputAmount.Currency.IsToken() && p.InvolvesToken(inputAmount.Currency.Wrapped())) {
		return nil, ErrTokenNotInvolved
	}
	zeroForOne := inputAmount.Currency.Equal(p.Token0)
	q, err := int256.FromBig(inputAmount.Quotient())
	if err != nil {
		return nil, err
	}
	swapResult := new(SwapResultV2)
	err = p.Swap(zeroForOne, q, sqrtPriceLimitX96, swapResult)
	if err != nil {
		return nil, err
	}
	var outputToken *entities.Token
	if zeroForOne {
		outputToken = p.Token1
	} else {
		outputToken = p.Token0
	}
	// pool, err := NewPoolV2(
	// 	p.Token0,
	// 	p.Token1,
	// 	p.Fee,
	// 	swapResult.SqrtRatioX96,
	// 	swapResult.Liquidity,
	// 	swapResult.CurrentTick,
	// 	p.TickDataProvider,
	// )

	pool := NewPoolV3(
		uint16(p.Fee),
		p.TickCurrent,
		p.SqrtRatioX96,
		p.Token0,
		p.Token1,
		p.TickDataProvider,
	)
	// if err != nil {
	// 	return nil, err
	// }
	return &GetAmountResult{
		ReturnedAmount:     entities.FromRawAmount(outputToken, new(utils.Int256).Neg(swapResult.AmountCalculated).ToBig()),
		RemainingAmountIn:  entities.FromRawAmount(inputAmount.Currency, swapResult.RemainingAmountIn.ToBig()),
		NewPoolState:       pool,
		CrossInitTickLoops: swapResult.CrossInitTickLoops,
	}, nil
}

func (p *Pool) GetOutputAmountV2(inputAmount *utils.Int256, zeroForOne bool,
	sqrtPriceLimitX96 *utils.Uint160) (*GetAmountResultV2, error) {

	swapResult := new(SwapResultV2)
	err := p.Swap(zeroForOne, inputAmount, sqrtPriceLimitX96, swapResult)
	if err != nil {
		return nil, err
	}
	return &GetAmountResultV2{
		ReturnedAmount:     new(utils.Int256).Neg(swapResult.AmountCalculated),
		RemainingAmountIn:  swapResult.RemainingAmountIn.Clone(),
		SqrtRatioX96:       swapResult.SqrtRatioX96,
		Liquidity:          swapResult.Liquidity,
		CurrentTick:        swapResult.CurrentTick,
		CrossInitTickLoops: swapResult.CrossInitTickLoops,
	}, nil
}

/**
 * Given a desired output amount of a token, return the computed input amount and a pool with state updated after the trade
 * @param outputAmount the output amount for which to quote the input amount
 * @param sqrtPriceLimitX96 The Q64.96 sqrt price limit. If zero for one, the price cannot be less than this value after the swap. If one for zero, the price cannot be greater than this value after the swap
 * @returns The input amount and the pool with updated state
 */
func (p *Pool) GetInputAmount(outputAmount *entities.CurrencyAmount,
	sqrtPriceLimitX96 *utils.Uint160) (*entities.CurrencyAmount, *Pool, error) {
	if !(outputAmount.Currency.IsToken() && p.InvolvesToken(outputAmount.Currency.Wrapped())) {
		return nil, nil, ErrTokenNotInvolved
	}
	zeroForOne := outputAmount.Currency.Equal(p.Token1)
	q, err := int256.FromBig(outputAmount.Quotient())
	if err != nil {
		return nil, nil, err
	}
	q.Neg(q)
	swapResult := new(SwapResultV2)
	err = p.Swap(zeroForOne, q, sqrtPriceLimitX96, swapResult)
	if err != nil {
		return nil, nil, err
	}
	var inputToken *entities.Token
	if zeroForOne {
		inputToken = p.Token0
	} else {
		inputToken = p.Token1
	}
	// pool, err := NewPoolV3(
	// 	p.Token0,
	// 	p.Token1,
	// 	p.Fee,
	// 	swapResult.sqrtRatioX96,
	// 	swapResult.liquidity,
	// 	swapResult.currentTick,
	// 	p.TickDataProvider,
	// )

	pool := NewPoolV3(
		uint16(p.Fee),
		p.TickCurrent,
		p.SqrtRatioX96,
		p.Token0,
		p.Token1,
		p.TickDataProvider,
	)
	// if err != nil {
	// 	return nil, nil, err
	// }
	return entities.FromRawAmount(inputToken, swapResult.AmountCalculated.ToBig()), pool, nil
}

/**
 * Executes a swap
 * @param zeroForOne Whether the amount in is token0 or token1
 * @param amountSpecified The amount of the swap, which implicitly configures the swap as exact input (positive), or exact output (negative)
 * @param sqrtPriceLimitX96 The Q64.96 sqrt price limit. If zero for one, the price cannot be less than this value after the swap. If one for zero, the price cannot be greater than this value after the swap
 * @returns swapResult.amountCalculated
 * @returns swapResult.sqrtRatioX96
 * @returns swapResult.liquidity
 * @returns swapResult.tickCurrent
 */
func (p *Pool) swap(zeroForOne bool, amountSpecified *utils.Int256, sqrtPriceLimitX96 *utils.Uint160) (*SwapResult, error) {
	panic("swap() is deprecated, use Swap() instead")

	// var err error
	// if sqrtPriceLimitX96 == nil {
	// 	if zeroForOne {
	// 		sqrtPriceLimitX96 = new(uint256.Int).AddUint64(utils.MinSqrtRatioU256, 1)
	// 	} else {
	// 		sqrtPriceLimitX96 = new(uint256.Int).SubUint64(utils.MaxSqrtRatioU256, 1)
	// 	}
	// }

	// if zeroForOne {
	// 	if sqrtPriceLimitX96.Lt(utils.MinSqrtRatioU256) {
	// 		return nil, ErrSqrtPriceLimitX96TooLow
	// 	}
	// 	if sqrtPriceLimitX96.Cmp(p.SqrtRatioX96) >= 0 {
	// 		return nil, ErrSqrtPriceLimitX96TooHigh
	// 	}
	// } else {
	// 	if sqrtPriceLimitX96.Gt(utils.MaxSqrtRatioU256) {
	// 		return nil, ErrSqrtPriceLimitX96TooHigh
	// 	}
	// 	if sqrtPriceLimitX96.Cmp(p.SqrtRatioX96) <= 0 {
	// 		return nil, ErrSqrtPriceLimitX96TooLow
	// 	}
	// }

	// exactInput := amountSpecified.Sign() >= 0

	// // keep track of swap state

	// state := struct {
	// 	amountSpecifiedRemaining *utils.Int256
	// 	amountCalculated         *utils.Int256
	// 	sqrtPriceX96             *utils.Uint160
	// 	tick                     int
	// 	liquidity                *utils.Uint128
	// }{
	// 	amountSpecifiedRemaining: new(utils.Int256).Set(amountSpecified),
	// 	amountCalculated:         int256.NewInt(0),
	// 	sqrtPriceX96:             new(utils.Uint160).Set(p.SqrtRatioX96),
	// 	tick:                     p.TickCurrent,
	// 	liquidity:                new(utils.Uint128).Set(p.Liquidity),
	// }

	// // crossInitTickLoops is the number of loops that cross an initialized tick.
	// // We only count when tick passes an initialized tick, since gas only significant in this case.
	// crossInitTickLoops := 0

	// // start swap while loop
	// for !state.amountSpecifiedRemaining.IsZero() && !state.sqrtPriceX96.Eq(sqrtPriceLimitX96) {
	// 	var step StepComputations
	// 	step.sqrtPriceStartX96.Set(state.sqrtPriceX96)

	// 	// because each iteration of the while loop rounds, we can't optimize this code (relative to the smart contract)
	// 	// by simply traversing to the next available tick, we instead need to exactly replicate
	// 	// tickBitmap.nextInitializedTickWithinOneWord
	// 	step.tickNext, step.initialized, err = p.TickDataProvider.NextInitializedTickIndex(state.tick, zeroForOne)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	if step.tickNext < utils.MinTick {
	// 		step.tickNext = utils.MinTick
	// 	} else if step.tickNext > utils.MaxTick {
	// 		step.tickNext = utils.MaxTick
	// 	}

	// 	p.tickCalculator.GetSqrtRatioAtTickV2(step.tickNext, &step.sqrtPriceNextX96)

	// 	var targetValue utils.Uint160
	// 	if zeroForOne {
	// 		if step.sqrtPriceNextX96.Lt(sqrtPriceLimitX96) {
	// 			targetValue.Set(sqrtPriceLimitX96)
	// 		} else {
	// 			targetValue.Set(&step.sqrtPriceNextX96)
	// 		}
	// 	} else {
	// 		if step.sqrtPriceNextX96.Gt(sqrtPriceLimitX96) {
	// 			targetValue.Set(sqrtPriceLimitX96)
	// 		} else {
	// 			targetValue.Set(&step.sqrtPriceNextX96)
	// 		}
	// 	}

	// 	var nxtSqrtPriceX96 utils.Uint160
	// 	err = p.swapStepCalculator.ComputeSwapStep(state.sqrtPriceX96, &targetValue, state.liquidity, state.amountSpecifiedRemaining,
	// 		p.Fee,
	// 		&nxtSqrtPriceX96, &step.amountIn, &step.amountOut, &step.feeAmount)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	state.sqrtPriceX96.Set(&nxtSqrtPriceX96)

	// 	var amountInPlusFee utils.Uint256
	// 	amountInPlusFee.Add(&step.amountIn, &step.feeAmount)

	// 	var amountInPlusFeeSigned utils.Int256
	// 	err = p.intTypes.ToInt256(&amountInPlusFee, &amountInPlusFeeSigned)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	var amountOutSigned utils.Int256
	// 	err = p.intTypes.ToInt256(&step.amountOut, &amountOutSigned)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	if exactInput {
	// 		state.amountSpecifiedRemaining.Sub(state.amountSpecifiedRemaining, &amountInPlusFeeSigned)
	// 		state.amountCalculated.Sub(state.amountCalculated, &amountOutSigned)
	// 	} else {
	// 		state.amountSpecifiedRemaining.Add(state.amountSpecifiedRemaining, &amountOutSigned)
	// 		state.amountCalculated.Add(state.amountCalculated, &amountInPlusFeeSigned)
	// 	}

	// 	// TODO
	// 	if state.sqrtPriceX96.Eq(&step.sqrtPriceNextX96) {
	// 		// if the tick is initialized, run the tick transition
	// 		if step.initialized {
	// 			tick, err := p.TickDataProvider.GetTick(step.tickNext)
	// 			if err != nil {
	// 				return nil, err
	// 			}

	// 			liquidityNet := tick.LiquidityNet
	// 			// if we're moving leftward, we interpret liquidityNet as the opposite sign
	// 			// safe because liquidityNet cannot be type(int128).min
	// 			if zeroForOne {
	// 				liquidityNet = new(utils.Int128).Neg(liquidityNet)
	// 			}
	// 			p.intTypes.AddDeltaInPlace(state.liquidity, liquidityNet)

	// 			crossInitTickLoops++
	// 		}
	// 		if zeroForOne {
	// 			state.tick = step.tickNext - 1
	// 		} else {
	// 			state.tick = step.tickNext
	// 		}

	// 	} else if !state.sqrtPriceX96.Eq(&step.sqrtPriceStartX96) {
	// 		// recompute unless we're on a lower tick boundary (i.e. already transitioned ticks), and haven't moved
	// 		state.tick, err = p.tickCalculator.GetTickAtSqrtRatioV2(state.sqrtPriceX96)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 	}
	// }

	// return &SwapResult{
	// 	amountCalculated:   state.amountCalculated,
	// 	sqrtRatioX96:       state.sqrtPriceX96,
	// 	liquidity:          state.liquidity,
	// 	currentTick:        state.tick,
	// 	remainingAmountIn:  state.amountSpecifiedRemaining,
	// 	crossInitTickLoops: crossInitTickLoops,
	// }, nil
}

func (p *Pool) tickSpacing() int {
	return constants.TickSpacings[p.Fee]
}

type StepFeeResult struct {
	ZeroForOne bool
	Tick       int
	FeeAmount  utils.Uint256
	AmountIn   utils.Uint256
	Liquidity  utils.Uint256
}

type SwapResultV2 struct {
	AmountCalculated   *utils.Int256
	SqrtRatioX96       *utils.Uint160
	Liquidity          *utils.Uint128
	RemainingAmountIn  *utils.Int256
	CurrentTick        int
	CrossInitTickLoops int
	StepsFee           []StepFeeResult
}

var (
	sqrtPriceLimitX96Upper = new(uint256.Int).AddUint64(utils.MinSqrtRatioU256, 1)
	sqrtPriceLimitX96Lower = new(uint256.Int).SubUint64(utils.MaxSqrtRatioU256, 1)
)

type State struct {
	amountSpecifiedRemaining *utils.Int256
	amountCalculated         *utils.Int256
	sqrtPriceX96             *utils.Uint160
	tick                     int
	liquidity                *utils.Uint128
}

var swapResultTmp = new(SwapResultV2)

func (p *Pool) Swap(zeroForOne bool, amountSpecified *utils.Int256, sqrtPriceLimitX96 *utils.Uint160, swapResult *SwapResultV2) error {
	var err error

	if sqrtPriceLimitX96 == nil {
		if zeroForOne {
			sqrtPriceLimitX96 = sqrtPriceLimitX96Upper
		} else {
			sqrtPriceLimitX96 = sqrtPriceLimitX96Lower
		}
	}

	if zeroForOne {
		if sqrtPriceLimitX96.Lt(utils.MinSqrtRatioU256) {
			return ErrSqrtPriceLimitX96TooLow
		}
		if !sqrtPriceLimitX96.Lt(p.SqrtRatioX96) {
			return ErrSqrtPriceLimitX96TooHigh
		}
	} else {
		if sqrtPriceLimitX96.Gt(utils.MaxSqrtRatioU256) {
			return ErrSqrtPriceLimitX96TooHigh
		}
		if !sqrtPriceLimitX96.Gt(p.SqrtRatioX96) {
			return ErrSqrtPriceLimitX96TooLow
		}
	}

	exactInput := amountSpecified.Sign() >= 0

	// keep track of swap state
	p.lastState.amountSpecifiedRemaining.Set(amountSpecified)
	p.lastState.amountCalculated.Clear()
	p.lastState.sqrtPriceX96.Set(p.SqrtRatioX96)
	p.lastState.tick = p.TickCurrent
	p.lastState.liquidity.Set(p.Liquidity)
	if swapResult == nil {
		swapResult = swapResultTmp
	}
	swapResult.StepsFee = []StepFeeResult{}
	swapResult.CrossInitTickLoops = 0

	// crossInitTickLoops is the number of loops that cross an initialized tick.
	// We only count when tick passes an initialized tick, since gas only significant in this case.
	// swapResult.CrossInitTickLoops = 0

	// start swap while loop
	for !p.lastState.amountSpecifiedRemaining.IsZero() && !p.lastState.sqrtPriceX96.Eq(sqrtPriceLimitX96) {
		p.step.sqrtPriceStartX96.Set(p.lastState.sqrtPriceX96)

		// because each iteration of the while loop rounds, we can't optimize this code (relative to the smart contract)
		// by simply traversing to the next available tick, we instead need to exactly replicate
		// tickBitmap.nextInitializedTickWithinOneWord
		p.step.tickNext, p.step.initialized, err = p.TickDataProvider.NextInitializedTickIndex(p.lastState.tick, zeroForOne)
		if err != nil {
			return err
		}

		if p.step.tickNext < utils.MinTick {
			p.step.tickNext = utils.MinTick
		} else if p.step.tickNext > utils.MaxTick {
			p.step.tickNext = utils.MaxTick
		}

		p.tickCalculator.GetSqrtRatioAtTickV2(p.step.tickNext, &p.step.sqrtPriceNextX96)

		if zeroForOne {
			if p.step.sqrtPriceNextX96.Lt(sqrtPriceLimitX96) {
				p.targetValue.Set(sqrtPriceLimitX96)
			} else {
				p.targetValue.Set(&p.step.sqrtPriceNextX96)
			}
		} else {
			if p.step.sqrtPriceNextX96.Gt(sqrtPriceLimitX96) {
				p.targetValue.Set(sqrtPriceLimitX96)
			} else {
				p.targetValue.Set(&p.step.sqrtPriceNextX96)
			}
		}

		if err = p.swapStepCalculator.ComputeSwapStep(p.lastState.sqrtPriceX96, p.targetValue, p.lastState.liquidity, p.lastState.amountSpecifiedRemaining, uint64(p.Fee), p.nxtSqrtPriceX96, &p.step.amountIn, &p.step.amountOut, &p.step.feeAmount); err != nil {
			return err
		}
		p.lastState.sqrtPriceX96.Set(p.nxtSqrtPriceX96)

		p.amountInPlusFee.Add(&p.step.amountIn, &p.step.feeAmount)

		if err = p.intTypes.ToInt256(p.amountInPlusFee, p.amountInPlusFeeSigned); err != nil {
			return err
		}

		if err = p.intTypes.ToInt256(&p.step.amountOut, p.amountOutSigned); err != nil {
			return err
		}

		swapResult.StepsFee = append(swapResult.StepsFee, StepFeeResult{
			Tick:       p.lastState.tick,
			FeeAmount:  p.step.feeAmount,
			AmountIn:   p.step.amountIn,
			ZeroForOne: zeroForOne,
			Liquidity:  *p.lastState.liquidity,
		})

		if exactInput {
			p.lastState.amountSpecifiedRemaining.Sub(p.lastState.amountSpecifiedRemaining, p.amountInPlusFeeSigned)
			p.lastState.amountCalculated.Sub(p.lastState.amountCalculated, p.amountOutSigned)
		} else {
			p.lastState.amountSpecifiedRemaining.Add(p.lastState.amountSpecifiedRemaining, p.amountOutSigned)
			p.lastState.amountCalculated.Add(p.lastState.amountCalculated, p.amountInPlusFeeSigned)
		}

		// TODO
		if p.lastState.sqrtPriceX96.Eq(&p.step.sqrtPriceNextX96) {
			// if the tick is initialized, run the tick transition
			if p.step.initialized {
				tick, err := p.TickDataProvider.GetTick(p.step.tickNext)
				if err != nil {
					return err
				}

				p.liquidityNet.Set(tick.LiquidityNet)

				// if we're moving leftward, we interpret liquidityNet as the opposite sign
				// safe because liquidityNet cannot be type(int128).min
				if zeroForOne {
					p.liquidityNet.Neg(p.liquidityNet)
				}
				p.intTypes.AddDeltaInPlace(p.lastState.liquidity, p.liquidityNet)

				swapResult.CrossInitTickLoops++
			}

			if zeroForOne {
				p.lastState.tick = p.step.tickNext - 1
			} else {
				p.lastState.tick = p.step.tickNext
			}

		} else if !p.lastState.sqrtPriceX96.Eq(&p.step.sqrtPriceStartX96) {
			// recompute unless we're on a lower tick boundary (i.e. already transitioned ticks), and haven't moved
			if p.lastState.tick, err = p.tickCalculator.GetTickAtSqrtRatioV2(p.lastState.sqrtPriceX96); err != nil {
				return err
			}
		}
	}

	swapResult.AmountCalculated = p.lastState.amountCalculated
	swapResult.SqrtRatioX96 = p.lastState.sqrtPriceX96
	swapResult.Liquidity = p.lastState.liquidity
	swapResult.CurrentTick = p.lastState.tick
	swapResult.RemainingAmountIn = p.lastState.amountSpecifiedRemaining

	return nil
}
