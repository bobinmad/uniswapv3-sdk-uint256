package entities

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/vuquang23/int256"
)

var benchmarkTickSearchIndex int

func binarySearchPreviousUpperBound(h *TicksHandler, tick int32) int {
	ticks := h.Ticks
	n := h.TicksLen
	if n <= 1 {
		return 0
	}
	pos, step := 0, n
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

func binarySearchHintCandidate(h *TicksHandler, tick int32, hint *int) int {
	ticks := h.Ticks
	lo, hi := 0, h.TicksLen
	if previous := *hint; previous >= 0 && previous < hi {
		if ticks[previous].Index <= tick {
			lo = previous
		} else {
			hi = previous + 1
		}
	}
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if ticks[mid].Index <= tick {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		*hint = 0
		return 0
	}
	*hint = lo - 1
	return lo - 1
}

func newTickSearchBenchmarkHandler(size int) *TicksHandler {
	ticks := make([]Tick, size)
	for i := range ticks {
		ticks[i] = Tick{
			Index:          -300_000 + int32(i*10),
			LiquidityGross: uint256.NewInt(1),
			LiquidityNet:   new(int256.Int),
		}
	}
	handler := NewTicksHandler()
	handler.SetTicks(ticks)
	return handler
}

func TestTicksHandlerBinarySearchMatchesPreviousUpperBound(t *testing.T) {
	for size := 1; size <= 257; size++ {
		handler := newTickSearchBenchmarkHandler(size)
		first := handler.Ticks[0].Index
		last := handler.Ticks[size-1].Index
		for tick := first - 11; tick <= last+11; tick++ {
			got := handler.binarySearch(tick)
			want := binarySearchPreviousUpperBound(handler, tick)
			if got != want {
				t.Fatalf("size=%d tick=%d got=%d want=%d", size, tick, got, want)
			}
		}
	}
}

func TestTicksHandlerDirectCacheCollisionAndIndexShifts(t *testing.T) {
	handler := newTickSearchBenchmarkHandler(128)
	cacheSlot := func(tick int32) uint16 {
		return uint16((uint32(tick) * 0x9e3779b1) >> 28)
	}

	firstIdx, secondIdx := -1, -1
	for i := 0; i < handler.TicksLen && firstIdx < 0; i++ {
		for j := i + 1; j < handler.TicksLen; j++ {
			if cacheSlot(handler.Ticks[i].Index) == cacheSlot(handler.Ticks[j].Index) {
				firstIdx, secondIdx = i, j
				break
			}
		}
	}
	if firstIdx < 0 {
		t.Fatal("test setup: expected a direct-cache collision")
	}

	for _, idx := range []int{firstIdx, secondIdx, firstIdx} {
		tick := handler.Ticks[idx].Index
		got, gotIdx, exists := handler.tickWithSliceKey(tick)
		if !exists || gotIdx != idx || got.Index != tick {
			t.Fatalf("collision lookup tick=%d: exists=%v idx=%d want=%d", tick, exists, gotIdx, idx)
		}
	}

	targetIdx := 100
	targetTick := handler.Ticks[targetIdx].Index
	targetSlot := cacheSlot(targetTick)
	_, _, _ = handler.tickWithSliceKey(targetTick)

	lowerIdx := 5
	lower := handler.Ticks[lowerIdx].Index + 1
	upper := handler.Ticks[lowerIdx+40].Index + 1
	for cacheSlot(lower) == targetSlot || cacheSlot(upper) == targetSlot {
		lowerIdx++
		lower = handler.Ticks[lowerIdx].Index + 1
		upper = handler.Ticks[lowerIdx+40].Index + 1
	}

	liquidity := uint256.NewInt(1)
	handler.UpdateTicksAfterMint(lower, upper, liquidity)
	if handler.cacheTicks[targetSlot] != targetTick || handler.cacheIdx[targetSlot] != int32(targetIdx+2) {
		t.Fatalf(
			"cached index after insert: tick=%d idx=%d, want tick=%d idx=%d",
			handler.cacheTicks[targetSlot],
			handler.cacheIdx[targetSlot],
			targetTick,
			targetIdx+2,
		)
	}

	handler.UpdateTicksAfterBurn(lower, upper, liquidity)
	if handler.cacheTicks[targetSlot] != targetTick || handler.cacheIdx[targetSlot] != int32(targetIdx) {
		t.Fatalf(
			"cached index after delete: tick=%d idx=%d, want tick=%d idx=%d",
			handler.cacheTicks[targetSlot],
			handler.cacheIdx[targetSlot],
			targetTick,
			targetIdx,
		)
	}
}

// BenchmarkTicksHandlerBinarySearchCandidates фиксирует основной оставшийся
// SDK-hotspot полного defisimulator-профиля. existing_tick моделирует lookup
// границ исторических Mint/Burn, insertion_point — добавление нового тика.
func BenchmarkTicksHandlerBinarySearchCandidates(b *testing.B) {
	handler := newTickSearchBenchmarkHandler(4_096)
	existingQueries := make([]int32, 4_096)
	insertionQueries := make([]int32, 4_096)
	var state uint32 = 0x9e3779b9
	for i := range existingQueries {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		index := int(state & 4_095)
		existingQueries[i] = handler.Ticks[index].Index
		insertionQueries[i] = handler.Ticks[index].Index + 5
	}

	algorithms := []struct {
		name   string
		search func(*TicksHandler, int32) int
	}{
		{name: "production", search: (*TicksHandler).binarySearch},
		{name: "previous_upper_bound", search: binarySearchPreviousUpperBound},
	}
	workloads := []struct {
		name    string
		queries []int32
	}{
		{name: "existing_tick", queries: existingQueries},
		{name: "insertion_point", queries: insertionQueries},
	}

	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			for _, algorithm := range algorithms {
				b.Run(algorithm.name, func(b *testing.B) {
					search := algorithm.search
					queries := workload.queries
					mask := len(queries) - 1
					result := 0
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						result += search(handler, queries[i&mask])
					}
					benchmarkTickSearchIndex = result
				})
			}
		})
	}
}

// BenchmarkTicksHandlerBinarySearchHintCandidate проверяет stateful finger
// search для последовательности lower/upper границ Mint/Burn. random_existing
// контролирует цену hint-а при отсутствии локальности.
func BenchmarkTicksHandlerBinarySearchHintCandidate(b *testing.B) {
	handler := newTickSearchBenchmarkHandler(4_096)
	randomQueries := make([]int32, 4_096)
	boundaryQueries := make([]int32, 4_096)
	var state uint32 = 0x85ebca6b
	for i := 0; i < len(randomQueries); i += 2 {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		index := int(state % uint32(len(handler.Ticks)-64))
		randomQueries[i] = handler.Ticks[index].Index
		randomQueries[i+1] = handler.Ticks[int(state)&4_095].Index
		boundaryQueries[i] = handler.Ticks[index].Index
		boundaryQueries[i+1] = handler.Ticks[index+51].Index
	}

	for _, workload := range []struct {
		name    string
		queries []int32
	}{
		{name: "random_existing", queries: randomQueries},
		{name: "mint_burn_boundaries", queries: boundaryQueries},
	} {
		b.Run(workload.name, func(b *testing.B) {
			b.Run("production", func(b *testing.B) {
				queries := workload.queries
				mask, result := len(queries)-1, 0
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					result += handler.binarySearch(queries[i&mask])
				}
				benchmarkTickSearchIndex = result
			})
			b.Run("hint_candidate", func(b *testing.B) {
				queries := workload.queries
				mask, result, hint := len(queries)-1, 0, -1
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					result += binarySearchHintCandidate(handler, queries[i&mask], &hint)
				}
				benchmarkTickSearchIndex = result
			})
		})
	}
}
