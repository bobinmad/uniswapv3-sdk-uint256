package entities

import (
	"testing"

	"github.com/bobinmad/uniswapv3-sdk-uint256/constants"
	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/vuquang23/int256"
)

// newBoundedTestPool строит пул с одной LP-позицией [lowerTick, upperTick) и
// стартовым тиком currentTick. Используется для проверок свапа, которые выходят
// за пределы инициализированных тиков (empty-zone behaviour).
func newBoundedTestPool(lowerTick, upperTick, currentTick int32, liquidity *uint256.Int) *Pool {
	L := int256.MustFromDec(liquidity.Dec())

	th := NewTicksHandler()
	th.SetTicks([]Tick{
		{Index: lowerTick, LiquidityGross: liquidity.Clone(), LiquidityNet: L.Clone()},
		{Index: upperTick, LiquidityGross: liquidity.Clone(), LiquidityNet: new(int256.Int).Neg(L)},
	})

	var sqrtP utils.Uint160
	utils.NewTickCalculator().GetSqrtRatioAtTickV2(currentTick, &sqrtP)

	p := NewPoolV3(common.Address{}, uint16(constants.FeeLow), currentTick, &sqrtP, USDC, DAI, th)
	// активная ликвидность есть только если текущий тик внутри диапазона позиции
	if currentTick >= lowerTick && currentTick < upperTick {
		p.Liquidity = liquidity.Clone()
	}
	return p
}

// TestSwap_OneForZero_ExitsUpThroughEmptyZone: exactInput swap "вверх" с бесконечно
// большим входом. Должен пересечь верхний тик, исчерпать всю ликвидность и без
// паники доехать до sqrtPriceLimitX96Lower (= MAX_SQRT_RATIO-1).
func TestSwap_OneForZero_ExitsUpThroughEmptyZone(t *testing.T) {
	L := uint256.NewInt(1e18)
	pool := newBoundedTestPool(-100, 100, 0, L)

	// слишком большой вход, чтобы гарантированно выйти за верхний тик
	amountIn, _ := int256.FromDec("1000000000000000000000000") // 1e24

	sr := &SwapResultV2{}
	err := pool.Swap(false, amountIn, nil, sr)
	assert.NoError(t, err, "swap не должен падать, даже если уходит в пустую зону сверху")

	expected := new(uint256.Int).SubUint64(utils.MaxSqrtRatioU256, 1)
	assert.True(t, sr.SqrtRatioX96.Eq(expected),
		"цена должна остановиться на sqrtPriceLimitX96Lower (MAX-1), got %s", sr.SqrtRatioX96.Dec())
	assert.True(t, sr.Liquidity.IsZero(), "ликвидность после выхода из всех позиций должна быть 0")
	assert.Equal(t, utils.MaxTick-1, sr.CurrentTick, "итоговый тик должен быть MaxTick-1")
	assert.False(t, sr.RemainingAmountIn.IsZero(),
		"остаток непотраченного входа должен быть > 0, так как в пустой зоне нечего свопать")
}

// TestSwap_ZeroForOne_ExitsDownThroughEmptyZone — симметрично предыдущему, но вниз.
func TestSwap_ZeroForOne_ExitsDownThroughEmptyZone(t *testing.T) {
	L := uint256.NewInt(1e18)
	pool := newBoundedTestPool(-100, 100, 0, L)

	amountIn, _ := int256.FromDec("1000000000000000000000000")

	sr := &SwapResultV2{}
	err := pool.Swap(true, amountIn, nil, sr)
	assert.NoError(t, err, "swap не должен падать, даже если уходит в пустую зону снизу")

	expected := new(uint256.Int).AddUint64(utils.MinSqrtRatioU256, 1)
	assert.True(t, sr.SqrtRatioX96.Eq(expected),
		"цена должна остановиться на sqrtPriceLimitX96Upper (MIN+1), got %s", sr.SqrtRatioX96.Dec())
	assert.True(t, sr.Liquidity.IsZero(), "ликвидность после выхода из всех позиций должна быть 0")
	assert.Equal(t, utils.MinTick, sr.CurrentTick, "итоговый тик должен быть MinTick")
	assert.False(t, sr.RemainingAmountIn.IsZero(),
		"остаток непотраченного входа должен быть > 0")
}

// TestSwap_OneForZero_StartsAtLargestTick: пул уже «над» всеми позициями
// (currentTick == LargestTickIdx, ликвидность = 0). Любой oneForZero свап
// должен без ошибок просто подвинуть цену до лимита.
func TestSwap_OneForZero_StartsAtLargestTick(t *testing.T) {
	L := uint256.NewInt(1e18)
	// currentTick = 100 == upper == LargestTickIdx → isAtOrAboveLargest == true
	pool := newBoundedTestPool(-100, 100, 100, L)
	assert.True(t, pool.Liquidity.IsZero(), "sanity: стартовая ликвидность должна быть 0")

	amountIn, _ := int256.FromDec("1000000000000000000")

	sr := &SwapResultV2{}
	err := pool.Swap(false, amountIn, nil, sr)
	assert.NoError(t, err, "ErrAtOrAboveLargest должен обрабатываться, а не всплывать из Swap")

	expected := new(uint256.Int).SubUint64(utils.MaxSqrtRatioU256, 1)
	assert.True(t, sr.SqrtRatioX96.Eq(expected), "цена должна уехать на верхний лимит")
	assert.True(t, sr.Liquidity.IsZero())
	// весь вход должен вернуться — в пустой зоне торговать нечем
	assert.True(t, sr.RemainingAmountIn.Eq(amountIn),
		"в пустой зоне ничего не свопается, весь вход должен остаться: got %s, want %s",
		sr.RemainingAmountIn.Dec(), amountIn.Dec())
}

// TestSwap_ZeroForOne_StartsBelowSmallestTick — симметрично: пул ниже всех
// позиций, zeroForOne свап не должен падать.
func TestSwap_ZeroForOne_StartsBelowSmallestTick(t *testing.T) {
	L := uint256.NewInt(1e18)
	// currentTick = -110 < lower = -100 → isBelowSmallest == true для lte=true
	pool := newBoundedTestPool(-100, 100, -110, L)
	assert.True(t, pool.Liquidity.IsZero(), "sanity: стартовая ликвидность должна быть 0")

	amountIn, _ := int256.FromDec("1000000000000000000")

	sr := &SwapResultV2{}
	err := pool.Swap(true, amountIn, nil, sr)
	assert.NoError(t, err, "ErrBelowSmallest должен обрабатываться, а не всплывать из Swap")

	expected := new(uint256.Int).AddUint64(utils.MinSqrtRatioU256, 1)
	assert.True(t, sr.SqrtRatioX96.Eq(expected), "цена должна уехать на нижний лимит")
	assert.True(t, sr.Liquidity.IsZero())
	assert.True(t, sr.RemainingAmountIn.Eq(amountIn),
		"весь вход должен остаться нетронутым")
}

// TestSwap_OneForZero_RespectsExplicitLimitBelowMaxTick: если при выходе в пустую
// зону торгующий задал собственный sqrtPriceLimitX96 < MAX, Swap должен
// останавливаться именно на нём, а не ехать до MAX.
func TestSwap_OneForZero_RespectsExplicitLimitBelowMaxTick(t *testing.T) {
	L := uint256.NewInt(1e18)
	pool := newBoundedTestPool(-100, 100, 0, L)

	// лимит цены = sqrtPriceAtTick(200) — это выше верхнего тика позиции,
	// но сильно ниже MAX_SQRT_RATIO
	var limit utils.Uint160
	utils.NewTickCalculator().GetSqrtRatioAtTickV2(200, &limit)

	amountIn, _ := int256.FromDec("1000000000000000000000000")

	sr := &SwapResultV2{}
	err := pool.Swap(false, amountIn, &limit, sr)
	assert.NoError(t, err)

	assert.True(t, sr.SqrtRatioX96.Eq(&limit),
		"swap должен остановиться ровно на пользовательском лимите, got %s, want %s",
		sr.SqrtRatioX96.Dec(), limit.Dec())
	assert.True(t, sr.Liquidity.IsZero(), "ликвидность всё равно исчерпана")
	assert.Equal(t, int32(200), sr.CurrentTick)
}

// TestNextInitializedTickIndex_AtLargestAndBelowSmallest проверяет инварианты
// самой TicksHandler: пограничные тики возвращают правильные ошибки, которые
// затем обрабатывает Pool.Swap.
func TestNextInitializedTickIndex_AtLargestAndBelowSmallest(t *testing.T) {
	L := uint256.NewInt(1e18)
	LI := int256.MustFromDec(L.Dec())

	th := NewTicksHandler()
	th.SetTicks([]Tick{
		{Index: -100, LiquidityGross: L.Clone(), LiquidityNet: LI.Clone()},
		{Index: 100, LiquidityGross: L.Clone(), LiquidityNet: new(int256.Int).Neg(LI)},
	})

	// lte=false в largest и выше — ErrAtOrAboveLargest
	_, _, err := th.NextInitializedTickIndex(100, false)
	assert.ErrorIs(t, err, ErrAtOrAboveLargest)
	_, _, err = th.NextInitializedTickIndex(500, false)
	assert.ErrorIs(t, err, ErrAtOrAboveLargest)

	// lte=true в smallest-1 и ниже — ErrBelowSmallest
	_, _, err = th.NextInitializedTickIndex(-101, true)
	assert.ErrorIs(t, err, ErrBelowSmallest)
	_, _, err = th.NextInitializedTickIndex(-500, true)
	assert.ErrorIs(t, err, ErrBelowSmallest)

	// внутри диапазона ошибок быть не должно
	idx, _, err := th.NextInitializedTickIndex(0, false)
	assert.NoError(t, err)
	assert.Equal(t, int32(100), idx)

	idx, _, err = th.NextInitializedTickIndex(0, true)
	assert.NoError(t, err)
	assert.Equal(t, int32(-100), idx)
}

// TestNextInitializedTickWithinOneWord_EmptyWords locks down the exact Core
// bitmap boundaries for a fee=0.3% pool (tickSpacing=60). In particular, the
// oneForZero boundary is an aligned compressed tick; using nextWord*spacing-1
// is an SDK approximation and changes swap rounding.
func TestNextInitializedTickWithinOneWord_EmptyWords(t *testing.T) {
	L := uint256.NewInt(1)
	th := NewTicksHandler()
	th.SetTicks([]Tick{
		{Index: -39120, LiquidityGross: L, LiquidityNet: int256.NewInt(1)},
		{Index: -6960, LiquidityGross: L, LiquidityNet: int256.NewInt(1)},
		{Index: -60, LiquidityGross: L, LiquidityNet: int256.NewInt(1)},
	})

	tests := []struct {
		name        string
		tick        int32
		lte         bool
		wantTick    int32
		initialized bool
	}{
		{name: "zeroForOne first empty word", tick: -6961, lte: true, wantTick: -15360},
		{name: "zeroForOne second empty word", tick: -15361, lte: true, wantTick: -30720},
		{name: "zeroForOne initialized tick", tick: -30721, lte: true, wantTick: -39120, initialized: true},
		{name: "oneForZero first empty word", tick: -37380, lte: false, wantTick: -30780},
		{name: "oneForZero second empty word", tick: -30780, lte: false, wantTick: -15420},
		{name: "oneForZero initialized tick", tick: -15420, lte: false, wantTick: -6960, initialized: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTick, gotInitialized, err := th.NextInitializedTickWithinOneWord(tt.tick, tt.lte, 60)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTick, gotTick)
			assert.Equal(t, tt.initialized, gotInitialized)
		})
	}
}

// TestSwap_MainnetEmptyBitmapWordRoundingRegression reproduces two consecutive
// swaps from Ethereum block 19,200,478 in pool Ee4C...818d. Both cross empty
// bitmap words. Skipping those words used to change amount/output by 1-2 wei.
func TestSwap_MainnetEmptyBitmapWordRoundingRegression(t *testing.T) {
	netLiquidities := []struct {
		tick int32
		net  uint64
	}{
		{tick: -39120, net: 58287719},
		{tick: -6960, net: 84330158},
		{tick: -6900, net: 57452803},
		{tick: -6840, net: 34458772},
		{tick: -2220, net: 39875074},
		{tick: -900, net: 4624204095},
		{tick: -120, net: 830252627174},
		{tick: -60, net: 1087164087529},
	}
	ticks := make([]Tick, 0, len(netLiquidities))
	for _, item := range netLiquidities {
		liquidity := uint256.NewInt(item.net)
		ticks = append(ticks, Tick{
			Index:          item.tick,
			LiquidityGross: liquidity,
			LiquidityNet:   int256.NewInt(int64(item.net)),
		})
	}
	handler := NewTicksHandler()
	handler.SetTicks(ticks)

	startPrice := uint256.MustFromDecimal("79099895125889336584214313780")
	pool := NewPoolV3(common.Address{}, uint16(constants.FeeMedium), -33, startPrice, USDC, DAI, handler)
	pool.Liquidity.SetUint64(1922315323324)

	firstAmount := int256.NewInt(5772385273)
	first := &SwapResultV2{}
	assert.NoError(t, pool.Swap(true, firstAmount, nil, first))
	assert.Equal(t, "-5419117338", first.AmountCalculated.Dec())
	assert.Equal(t, "12224680419793842286177254660", first.SqrtRatioX96.Dec())
	assert.Equal(t, "58287719", first.Liquidity.Dec())
	assert.Equal(t, int32(-37380), first.CurrentTick)
	assert.True(t, first.RemainingAmountIn.IsZero())

	pool.SqrtRatioX96.Set(first.SqrtRatioX96)
	pool.Liquidity.Set(first.Liquidity)
	pool.TickCurrent = first.CurrentTick

	secondAmount := int256.NewInt(401185315)
	second := &SwapResultV2{}
	assert.NoError(t, pool.Swap(false, secondAmount, nil, second))
	assert.Equal(t, "-702573661", second.AmountCalculated.Dec())
	assert.Equal(t, "78765700989611136151187343876", second.SqrtRatioX96.Dec())
	assert.Equal(t, "835151235795", second.Liquidity.Dec())
	assert.Equal(t, int32(-118), second.CurrentTick)
	assert.True(t, second.RemainingAmountIn.IsZero())
}
