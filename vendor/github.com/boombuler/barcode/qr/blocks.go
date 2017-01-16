package qr

type block struct {
	data []byte
	ecc  []byte
}
type blockList []*block

func splitToBlocks(data <-chan byte, vi *versionInfo) blockList {
	result := make(blockList, vi.NumberOfBlocksInGroup1+vi.NumberOfBlocksInGroup2)

	for b := 0; b < int(vi.NumberOfBlocksInGroup1); b++ {
		blk := new(block)
		blk.data = make([]byte, vi.DataCodeWordsPerBlockInGroup1)
		for cw := 0; cw < int(vi.DataCodeWordsPerBlockInGroup1); cw++ {
			blk.data[cw] = <-data
		}
		blk.ecc = ec.calcECC(blk.data, vi.ErrorCorrectionCodewordsPerBlock)
		result[b] = blk
	}

	for b := 0; b < int(vi.NumberOfBlocksInGroup2); b++ {
		blk := new(block)
		blk.data = make([]byte, vi.DataCodeWordsPerBlockInGroup2)
		for cw := 0; cw < int(vi.DataCodeWordsPerBlockInGroup2); cw++ {
			blk.data[cw] = <-data
		}
		blk.ecc = ec.calcECC(blk.data, vi.ErrorCorrectionCodewordsPerBlock)
		result[int(vi.NumberOfBlocksInGroup1)+b] = blk
	}

	return result
}

func (bl blockList) interleave(vi *versionInfo) []byte {
	var maxCodewordCount int
	if vi.DataCodeWordsPerBlockInGroup1 > vi.DataCodeWordsPerBlockInGroup2 {
		maxCodewordCount = int(vi.DataCodeWordsPerBlockInGroup1)
	} else {
		maxCodewordCount = int(vi.DataCodeWordsPerBlockInGroup2)
	}
	resultLen := (vi.DataCodeWordsPerBlockInGroup1+vi.ErrorCorrectionCodewordsPerBlock)*vi.NumberOfBlocksInGroup1 +
		(vi.DataCodeWordsPerBlockInGroup2+vi.ErrorCorrectionCodewordsPerBlock)*vi.NumberOfBlocksInGroup2

	result := make([]byte, 0, resultLen)
	for i := 0; i < maxCodewordCount; i++ {
		for b := 0; b < len(bl); b++ {
			if len(bl[b].data) > i {
				result = append(result, bl[b].data[i])
			}
		}
	}
	for i := 0; i < int(vi.ErrorCorrectionCodewordsPerBlock); i++ {
		for b := 0; b < len(bl); b++ {
			result = append(result, bl[b].ecc[i])
		}
	}
	return result
}
