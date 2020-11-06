package utils

// GaloisField encapsulates galois field arithmetics
type GaloisField struct {
	Size    int
	Base    int
	ALogTbl []int
	LogTbl  []int
}

// NewGaloisField creates a new galois field
func NewGaloisField(pp, fieldSize, b int) *GaloisField {
	result := new(GaloisField)

	result.Size = fieldSize
	result.Base = b
	result.ALogTbl = make([]int, fieldSize)
	result.LogTbl = make([]int, fieldSize)

	x := 1
	for i := 0; i < fieldSize; i++ {
		result.ALogTbl[i] = x
		x = x * 2
		if x >= fieldSize {
			x = (x ^ pp) & (fieldSize - 1)
		}
	}

	for i := 0; i < fieldSize; i++ {
		result.LogTbl[result.ALogTbl[i]] = int(i)
	}

	return result
}

func (gf *GaloisField) Zero() *GFPoly {
	return NewGFPoly(gf, []int{0})
}

// AddOrSub add or substract two numbers
func (gf *GaloisField) AddOrSub(a, b int) int {
	return a ^ b
}

// Multiply multiplys two numbers
func (gf *GaloisField) Multiply(a, b int) int {
	if a == 0 || b == 0 {
		return 0
	}
	return gf.ALogTbl[(gf.LogTbl[a]+gf.LogTbl[b])%(gf.Size-1)]
}

// Divide divides two numbers
func (gf *GaloisField) Divide(a, b int) int {
	if b == 0 {
		panic("divide by zero")
	} else if a == 0 {
		return 0
	}
	return gf.ALogTbl[(gf.LogTbl[a]-gf.LogTbl[b])%(gf.Size-1)]
}

func (gf *GaloisField) Invers(num int) int {
	return gf.ALogTbl[(gf.Size-1)-gf.LogTbl[num]]
}
