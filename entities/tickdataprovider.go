package entities

import (
	"slices"

	"github.com/KyberNetwork/int256"
	"github.com/bobinmad/uniswapv3-sdk-uint256/utils"
	"github.com/holiman/uint256"
)

type Tick struct {
	Index          int32
	LiquidityGross *uint256.Int
	LiquidityNet   *utils.Int128
}

// наш собственный расширенный TickListDataProvider с блэкджеком и шлюхами
type TicksHandler struct {
	Ticks []Tick

	// performance sake
	TicksLen        int
	SmallestTickIdx int32
	LargestTickIdx  int32
}

func NewTicksHandler() *TicksHandler {
	return &TicksHandler{}
}

// клонирует текущий тикхандлер путём создания глубокой копии.
// используется для создания нового тикхандлера каждой стратегии в многопотоке.
func (h *TicksHandler) Clone() *TicksHandler {
	ticksHandler := NewTicksHandler()

	ticksHandler.CloneTicks(h.Ticks)
	return ticksHandler
}

func (h *TicksHandler) SetTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross,
			LiquidityNet:   tick.LiquidityNet,
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
}

func (h *TicksHandler) CloneTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	// a kinda deep copy of ticks
	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross.Clone(),
			LiquidityNet:   tick.LiquidityNet.Clone(),
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
}

func (h *TicksHandler) GetTick(tick int32) (Tick, error) {
	i := h.binarySearch(tick)
	if h.Ticks[i].Index == tick {
		return h.Ticks[i], nil
	}
	return EmptyTick, ErrTickNotFound
}

func (h *TicksHandler) NextInitializedTickIndex(tick int32, lte bool) (int32, bool, error) {
	var idx int32
	var init bool

	if lte {
		if h.isBelowSmallest(tick) {
			return ZeroValueTickIndex, false, ErrBelowSmallest
		}
		if h.isAtOrAboveLargest(tick) {
			tick := &h.Ticks[h.TicksLen-1]
			idx, init = tick.Index, !tick.LiquidityGross.IsZero()
		} else {
			i := h.binarySearch(tick)
			tick := &h.Ticks[i]
			idx, init = tick.Index, !tick.LiquidityGross.IsZero()
		}
	} else {
		if h.isAtOrAboveLargest(tick) {
			return ZeroValueTickIndex, false, ErrAtOrAboveLargest
		}
		if h.isBelowSmallest(tick) {
			tick := &h.Ticks[0]
			idx, init = tick.Index, !tick.LiquidityGross.IsZero()
		} else {
			i := h.binarySearch(tick) + 1
			tick := &h.Ticks[i]
			idx, init = tick.Index, !tick.LiquidityGross.IsZero()
		}
	}

	return idx, init, nil
}

// актуализирует состояние тиков пула после историчекого события mint
func (h *TicksHandler) UpdateTicksAfterMint(tickLower, tickUpper int32, liquidity *uint256.Int) {
	liquidityI256 := (*int256.Int)(liquidity)

	if tick, sliceKey, exist := h.tickWithSliceKey(tickLower); exist {
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Add(tick.LiquidityNet, liquidityI256)
	} else {
		h.Ticks = slices.Insert(h.Ticks, sliceKey, Tick{Index: tickLower, LiquidityGross: liquidity.Clone(), LiquidityNet: liquidityI256.Clone()})
		h.TicksLen++
		if tickLower < h.SmallestTickIdx {
			h.SmallestTickIdx = tickLower
		}
	}

	if tick, sliceKey, exist := h.tickWithSliceKey(tickUpper); exist {
		tick.LiquidityGross.Add(tick.LiquidityGross, liquidity)
		tick.LiquidityNet.Sub(tick.LiquidityNet, liquidityI256)
	} else {
		h.Ticks = slices.Insert(h.Ticks, sliceKey, Tick{Index: tickUpper, LiquidityGross: liquidity.Clone(), LiquidityNet: new(int256.Int).Neg(liquidityI256)})
		h.TicksLen++
		if tickUpper > h.LargestTickIdx {
			h.LargestTickIdx = tickUpper
		}
	}
}

// актуализирует состояние тиков пула после историчекого события burn
func (h *TicksHandler) UpdateTicksAfterBurn(tickLower, tickUpper int32, liquidity *uint256.Int) {
	liquidityI256 := (*int256.Int)(liquidity)

	tick, sliceKey, _ := h.tickWithSliceKey(tickLower)
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Sub(tick.LiquidityNet, liquidityI256)
	h.removeTickIfEmpty(tick, sliceKey)

	tick, sliceKey, _ = h.tickWithSliceKey(tickUpper)
	tick.LiquidityGross.Sub(tick.LiquidityGross, liquidity)
	tick.LiquidityNet.Add(tick.LiquidityNet, liquidityI256)
	h.removeTickIfEmpty(tick, sliceKey)
}

// проверяет тик на пустую ликвидность и удаляет в таком случае
func (h *TicksHandler) removeTickIfEmpty(tick *Tick, sliceKey int) {
	if tick.LiquidityGross.IsZero() && tick.LiquidityNet.IsZero() {
		h.Ticks = slices.Delete(h.Ticks, sliceKey, sliceKey+1)
		h.TicksLen--

		if h.TicksLen > 0 {
			h.SmallestTickIdx = h.Ticks[0].Index
			h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
		}
	}
}

// tickWithSliceKey возвращает указатель на тик и индекс при найденном, иначе (nil, insertionIdx, false).
func (h *TicksHandler) tickWithSliceKey(tick int32) (*Tick, int, bool) {
	if h.TicksLen == 0 {
		return nil, 0, false
	}
	i := h.binarySearch(tick)
	idx := i
	if h.Ticks[i].Index < tick {
		idx = i + 1
	}
	if idx < h.TicksLen && h.Ticks[idx].Index == tick {
		return &h.Ticks[idx], idx, true
	}
	return nil, idx, false
}

func (h *TicksHandler) binarySearch(tick int32) int {
	start := 0
	end := h.TicksLen - 1

	for start < end {
		mid := start + (end-start)>>1
		if h.Ticks[mid].Index <= tick {
			start = mid + 1
		} else {
			end = mid
		}
	}
	// start == end: rightmost index with Ticks[i].Index <= tick
	if h.Ticks[start].Index <= tick {
		return start
	}
	if start > 0 {
		return start - 1
	}
	return 0
}

func (h *TicksHandler) isBelowSmallest(tick int32) bool {
	return tick < h.SmallestTickIdx
}

func (h *TicksHandler) isAtOrAboveLargest(tick int32) bool {
	return tick >= h.LargestTickIdx
}
