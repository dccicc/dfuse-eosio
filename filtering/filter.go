package filtering

import (
	"fmt"

	"github.com/dfuse-io/bstream"
	pbcodec "github.com/dfuse-io/dfuse-eosio/pb/dfuse/eosio/codec/v1"
)

type BlockFilter struct {
	IncludeProgram *CELFilter
	ExcludeProgram *CELFilter
}

func NewBlockFilter(includeProgramCode, excludeProgramCode string) (*BlockFilter, error) {
	includeFilter, err := newCELFilterInclude(includeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("include filter: %w", err)
	}

	excludeFilter, err := newCELFilterExclude(excludeProgramCode)
	if err != nil {
		return nil, fmt.Errorf("exclude filter: %w", err)
	}

	return &BlockFilter{
		IncludeProgram: includeFilter,
		ExcludeProgram: excludeFilter,
	}, nil
}

// TransformInPlace received a `bstream.Block` pointer, unpack it's native counterpart, a `pbcodec.Block` pointer
// in our case and transforms it in place, modifiying the pointed object. This means that future `ToNative()` calls
// on the bstream block will return a filtered version of this block.
//
// *Important* This method expect that the caller will peform the transformation in lock step, there is no lock
//             performed by this method. It's the caller responsibility to deal with concurrency issues.
func (f *BlockFilter) TransformInPlace(blk *bstream.Block) error {
	// Don't decode the bstream block at all so we save a costly unpacking when both filters are no-op filters
	if f.IncludeProgram.IsNoop() && f.ExcludeProgram.IsNoop() {
		return nil
	}

	f.transfromInPlace(blk.ToNative().(*pbcodec.Block))
	return nil
}

func (f *BlockFilter) transfromInPlace(block *pbcodec.Block) {
	block.FilteringApplied = true
	block.FilteringIncludeFilterExpr = f.IncludeProgram.code
	block.FilteringExcludeFilterExpr = f.ExcludeProgram.code

	var filteredTrxTrace []*pbcodec.TransactionTrace
	filteredExecutedInputActionCount := uint32(0)
	filteredExecutedTotalActionCount := uint32(0)

	for _, trxTrace := range block.UnfilteredTransactionTraces {
		trxTraceAddedToFiltered := false
		for _, actTrace := range trxTrace.ActionTraces {
			if !f.shouldProcess(trxTrace, actTrace) {
				continue
			}

			actTrace.FilteringMatched = true
			filteredExecutedTotalActionCount++
			if actTrace.IsInput() {
				filteredExecutedInputActionCount++
			}

			if !trxTraceAddedToFiltered {
				filteredTrxTrace = append(filteredTrxTrace, trxTrace)
				trxTraceAddedToFiltered = true
			}
		}

		if trxTrace.FailedDtrxTrace != nil {
			for _, actTrace := range trxTrace.FailedDtrxTrace.ActionTraces {
				if !f.shouldProcess(trxTrace.FailedDtrxTrace, actTrace) {
					continue
				}

				actTrace.FilteringMatched = true
				if !trxTraceAddedToFiltered {
					filteredTrxTrace = append(filteredTrxTrace, trxTrace)
					trxTraceAddedToFiltered = true
				}
			}
		}
	}

	block.UnfilteredTransactionTraces = nil
	block.FilteredTransactionTraces = filteredTrxTrace
	block.FilteredTransactionTraceCount = uint32(len(filteredTrxTrace))
	block.FilteredExecutedInputActionCount = filteredExecutedInputActionCount
	block.FilteredExecutedTotalActionCount = filteredExecutedTotalActionCount
}

func (f *BlockFilter) shouldProcess(trxTrace *pbcodec.TransactionTrace, actTrace *pbcodec.ActionTrace) bool {
	activation := actionTraceActivation{trace: actTrace, trxScheduled: trxTrace.Scheduled}
	// If the include program does not match, there is nothing more to do here
	if !f.IncludeProgram.match(&activation) {
		return false
	}

	// At this point, the inclusion expr matched, let's check it was included but should be now excluded based on the exclusion filter
	if f.ExcludeProgram.match(&activation) {
		return false
	}

	// We are included and NOT excluded, this transaction trace/action trace match the block filter
	return true
}
