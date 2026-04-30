package entities

import (
	"slices"

	"github.com/vuquang23/int256"
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
	Ticks      []Tick
	TicksLen   int
	SmallestTickIdx int32
	LargestTickIdx  int32

	// lastResultIdx кэширует slice-индекс последнего тика из NextInitializedTickIndex.
	// Используется двояко:
	//   1. GetTick: если Ticks[lastResultIdx].Index совпадает — binarySearch не нужен.
	//   2. NextInitializedTickIndex: sequential hint — внутри одного свапа тики
	//      пересекаются по одному (±1 шаг), поэтому проверяем соседний элемент
	//      перед запуском полного binary search (O(1) vs O(log N)).
	// Хранится как int (не pointer) — нет GC write barrier при каждом присваивании.
	// -1 означает «кэш невалиден» (после Mint/Burn и при инициализации).
	lastResultIdx int

	// P11: 4-way LRU-кэш для tickWithSliceKey.
	// В DeFi-стратегиях типичен паттерн "Mint(L,U)" + "Burn(L,U)" в тех же тиках (rebalance, fee-collect),
	// и Mint/Burn handler делает 2 lookup'а на одно событие. 4 ячеек хватает на 2 пары tickLower/tickUpper.
	// При insert/delete индексы автоматически смещаются (смещение в рамках invalidation).
	// 0 — sentinel "ячейка не валидна" (т.к. tick=0 редок, проверяем по cacheValidMask).
	cacheTicks     [4]int32 // искомый tick
	cacheIdx       [4]int32 // sliceKey в Ticks
	cacheValidMask uint8    // битмаска: 1<<i = ячейка i валидна
	cacheNext      uint8    // next slot для round-robin replacement
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
	h.lastResultIdx = -1
	h.cacheValidMask = 0
}

func (h *TicksHandler) CloneTicks(ticks []Tick) {
	h.TicksLen = len(ticks)
	h.Ticks = make([]Tick, h.TicksLen)

	for idx, tick := range ticks {
		h.Ticks[idx] = Tick{
			Index:          tick.Index,
			LiquidityGross: tick.LiquidityGross.Clone(),
			LiquidityNet:   tick.LiquidityNet.Clone(),
		}
	}

	h.SmallestTickIdx = h.Ticks[0].Index
	h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
	h.lastResultIdx = -1
	h.cacheValidMask = 0
}

func (h *TicksHandler) GetTick(tick int32) (Tick, error) {
	// Быстрый путь: если последний вызов NextInitializedTickIndex вернул именно этот тик,
	// возвращаем его без binary search. lastResultIdx — int, нет GC write barrier.
	if i := h.lastResultIdx; i >= 0 && h.Ticks[i].Index == tick {
		return h.Ticks[i], nil
	}
	i := h.binarySearch(tick)
	if h.Ticks[i].Index == tick {
		return h.Ticks[i], nil
	}
	return EmptyTick, ErrTickNotFound
}

func (h *TicksHandler) NextInitializedTickIndex(tick int32, lte bool) (int32, bool, error) {
	var i int

	if lte {
		if h.isBelowSmallest(tick) {
			return ZeroValueTickIndex, false, ErrBelowSmallest
		}
		if h.isAtOrAboveLargest(tick) {
			i = h.TicksLen - 1
		} else {
			// Sequential hint: внутри одного свапа тики пересекаются по одному (lte=true → индекс -=1).
			// Проверяем Ticks[last-1] перед полным binary search — O(1) вместо O(log N).
			hint := h.lastResultIdx - 1
			if hint >= 0 && h.Ticks[hint].Index <= tick && h.Ticks[h.lastResultIdx].Index > tick {
				i = hint
			} else {
				i = h.binarySearch(tick)
			}
		}
	} else {
		if h.isAtOrAboveLargest(tick) {
			return ZeroValueTickIndex, false, ErrAtOrAboveLargest
		}
		if h.isBelowSmallest(tick) {
			i = 0
		} else {
			// Sequential hint для lte=false: тики пересекаются по одному (индекс +=1).
			hint := h.lastResultIdx + 1
			if h.lastResultIdx >= 0 && hint < h.TicksLen &&
				h.Ticks[h.lastResultIdx].Index <= tick && h.Ticks[hint].Index > tick {
				i = hint
			} else {
				i = h.binarySearch(tick) + 1
			}
		}
	}

	// Кэшируем slice-индекс: используется в GetTick и как hint для следующего вызова.
	// int, не pointer — нет GC write barrier.
	h.lastResultIdx = i
	t := &h.Ticks[i]
	return t.Index, !t.LiquidityGross.IsZero(), nil
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
		h.shiftIndicesAfterInsert(int32(sliceKey))
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
		h.shiftIndicesAfterInsert(int32(sliceKey))
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
		h.shiftIndicesAfterDelete(int32(sliceKey))

		if h.TicksLen > 0 {
			h.SmallestTickIdx = h.Ticks[0].Index
			h.LargestTickIdx = h.Ticks[h.TicksLen-1].Index
		}
	}
}

// shiftIndicesAfterInsert корректирует кэшированные индексы после slices.Insert(at).
// Все индексы >= at сдвигаются на +1; sequential-hint lastResultIdx тоже обновляется.
//
//go:nosplit
func (h *TicksHandler) shiftIndicesAfterInsert(at int32) {
	mask := h.cacheValidMask
	for i := uint8(0); i < 4; i++ {
		if mask&(1<<i) != 0 && h.cacheIdx[i] >= at {
			h.cacheIdx[i]++
		}
	}
	if h.lastResultIdx >= int(at) {
		h.lastResultIdx++
	}
}

// shiftIndicesAfterDelete корректирует кэшированные индексы после slices.Delete(at).
// Удалённая ячейка инвалидируется; индексы > at сдвигаются на -1.
//
//go:nosplit
func (h *TicksHandler) shiftIndicesAfterDelete(at int32) {
	mask := h.cacheValidMask
	for i := uint8(0); i < 4; i++ {
		if mask&(1<<i) != 0 {
			if h.cacheIdx[i] == at {
				mask &^= 1 << i // удалённая ячейка — инвалидируем
			} else if h.cacheIdx[i] > at {
				h.cacheIdx[i]--
			}
		}
	}
	h.cacheValidMask = mask
	if h.lastResultIdx == int(at) {
		h.lastResultIdx = -1
	} else if h.lastResultIdx > int(at) {
		h.lastResultIdx--
	}
}

// tickWithSliceKey возвращает указатель на тик и индекс при найденном, иначе (nil, insertionIdx, false).
//
// 4-way LRU-кэш для типичного rebalance-паттерна Mint(L,U) → Burn(L,U) с одинаковыми тиками.
// Инвалидирует sequential-hint lastResultIdx (insert/delete смещает layout).
//
//go:nosplit
func (h *TicksHandler) tickWithSliceKey(tick int32) (*Tick, int, bool) {
	if h.TicksLen == 0 {
		return nil, 0, false
	}

	// fast path: 4-way кэш (развёрнутый луп — ~30% быстрее, чем for+mask:
	// branch-prediction лучше, и компилятор может выгрузить cacheTicks[0..3]
	// в регистры одним блоком).
	if mask := h.cacheValidMask; mask != 0 {
		ticksLen := h.TicksLen
		ticks := h.Ticks
		ct := &h.cacheTicks
		ci := &h.cacheIdx
		if mask&1 != 0 && ct[0] == tick {
			idx := int(ci[0])
			if idx < ticksLen && ticks[idx].Index == tick {
				return &ticks[idx], idx, true
			}
			h.cacheValidMask &^= 1
		} else if mask&2 != 0 && ct[1] == tick {
			idx := int(ci[1])
			if idx < ticksLen && ticks[idx].Index == tick {
				return &ticks[idx], idx, true
			}
			h.cacheValidMask &^= 2
		} else if mask&4 != 0 && ct[2] == tick {
			idx := int(ci[2])
			if idx < ticksLen && ticks[idx].Index == tick {
				return &ticks[idx], idx, true
			}
			h.cacheValidMask &^= 4
		} else if mask&8 != 0 && ct[3] == tick {
			idx := int(ci[3])
			if idx < ticksLen && ticks[idx].Index == tick {
				return &ticks[idx], idx, true
			}
			h.cacheValidMask &^= 8
		}
	}

	// ВАЖНО: lastResultIdx больше не сбрасываем здесь.
	// shiftIndicesAfterInsert/Delete сами корректно поддерживают индекс при insert/delete.
	// Если Mint попал в exist'ующий тик (без insert) — sequential hint следующего swap'а валиден.
	i := h.binarySearch(tick)
	idx := i
	if h.Ticks[i].Index < tick {
		idx = i + 1
	}
	if idx < h.TicksLen && h.Ticks[idx].Index == tick {
		// записываем в кэш round-robin replacement
		slot := h.cacheNext & 3
		h.cacheTicks[slot] = tick
		h.cacheIdx[slot] = int32(idx)
		h.cacheValidMask |= 1 << slot
		h.cacheNext = slot + 1
		return &h.Ticks[idx], idx, true
	}
	return nil, idx, false
}

// binarySearch возвращает наибольший индекс i, при котором Ticks[i].Index <= tick.
// Если все Ticks[i].Index > tick, возвращает 0 (для совместимости со старой семантикой).
//
// Реализация — branchless upper_bound (std::upper_bound из C++ STL):
// на каждой итерации шаг гарантированно уменьшается, цикл заканчивается через ~log2(N) итераций
// без непредсказуемых ветвей, которые в random-like binary search дают
// ~15-20 цикл. mispredict penalty.
//
//go:nosplit
func (h *TicksHandler) binarySearch(tick int32) int {
	ticks := h.Ticks
	n := h.TicksLen
	if n <= 1 {
		return 0
	}

	// upper_bound: ищем наименьший pos, где ticks[pos].Index > tick (или pos=n если такого нет).
	pos := 0
	step := n
	for step > 0 {
		half := step >> 1
		mid := pos + half
		if mid < n && ticks[mid].Index <= tick {
			pos = mid + 1
			step = step - half - 1
		} else {
			step = half
		}
	}
	if pos == 0 {
		return 0
	}
	return pos - 1
}

func (h *TicksHandler) isBelowSmallest(tick int32) bool {
	return tick < h.SmallestTickIdx
}

func (h *TicksHandler) isAtOrAboveLargest(tick int32) bool {
	return tick >= h.LargestTickIdx
}
