package utils

type GFPoly struct {
	gf           *GaloisField
	Coefficients []int
}

func (gp *GFPoly) Degree() int {
	return len(gp.Coefficients) - 1
}

func (gp *GFPoly) Zero() bool {
	return gp.Coefficients[0] == 0
}

// GetCoefficient returns the coefficient of x ^ degree
func (gp *GFPoly) GetCoefficient(degree int) int {
	return gp.Coefficients[gp.Degree()-degree]
}

func (gp *GFPoly) AddOrSubstract(other *GFPoly) *GFPoly {
	if gp.Zero() {
		return other
	} else if other.Zero() {
		return gp
	}
	smallCoeff := gp.Coefficients
	largeCoeff := other.Coefficients
	if len(smallCoeff) > len(largeCoeff) {
		largeCoeff, smallCoeff = smallCoeff, largeCoeff
	}
	sumDiff := make([]int, len(largeCoeff))
	lenDiff := len(largeCoeff) - len(smallCoeff)
	copy(sumDiff, largeCoeff[:lenDiff])
	for i := lenDiff; i < len(largeCoeff); i++ {
		sumDiff[i] = int(gp.gf.AddOrSub(int(smallCoeff[i-lenDiff]), int(largeCoeff[i])))
	}
	return NewGFPoly(gp.gf, sumDiff)
}

func (gp *GFPoly) MultByMonominal(degree int, coeff int) *GFPoly {
	if coeff == 0 {
		return gp.gf.Zero()
	}
	size := len(gp.Coefficients)
	result := make([]int, size+degree)
	for i := 0; i < size; i++ {
		result[i] = int(gp.gf.Multiply(int(gp.Coefficients[i]), int(coeff)))
	}
	return NewGFPoly(gp.gf, result)
}

func (gp *GFPoly) Multiply(other *GFPoly) *GFPoly {
	if gp.Zero() || other.Zero() {
		return gp.gf.Zero()
	}
	aCoeff := gp.Coefficients
	aLen := len(aCoeff)
	bCoeff := other.Coefficients
	bLen := len(bCoeff)
	product := make([]int, aLen+bLen-1)
	for i := 0; i < aLen; i++ {
		ac := int(aCoeff[i])
		for j := 0; j < bLen; j++ {
			bc := int(bCoeff[j])
			product[i+j] = int(gp.gf.AddOrSub(int(product[i+j]), gp.gf.Multiply(ac, bc)))
		}
	}
	return NewGFPoly(gp.gf, product)
}

func (gp *GFPoly) Divide(other *GFPoly) (quotient *GFPoly, remainder *GFPoly) {
	quotient = gp.gf.Zero()
	remainder = gp
	fld := gp.gf
	denomLeadTerm := other.GetCoefficient(other.Degree())
	inversDenomLeadTerm := fld.Invers(int(denomLeadTerm))
	for remainder.Degree() >= other.Degree() && !remainder.Zero() {
		degreeDiff := remainder.Degree() - other.Degree()
		scale := int(fld.Multiply(int(remainder.GetCoefficient(remainder.Degree())), inversDenomLeadTerm))
		term := other.MultByMonominal(degreeDiff, scale)
		itQuot := NewMonominalPoly(fld, degreeDiff, scale)
		quotient = quotient.AddOrSubstract(itQuot)
		remainder = remainder.AddOrSubstract(term)
	}
	return
}

func NewMonominalPoly(field *GaloisField, degree int, coeff int) *GFPoly {
	if coeff == 0 {
		return field.Zero()
	}
	result := make([]int, degree+1)
	result[0] = coeff
	return NewGFPoly(field, result)
}

func NewGFPoly(field *GaloisField, coefficients []int) *GFPoly {
	for len(coefficients) > 1 && coefficients[0] == 0 {
		coefficients = coefficients[1:]
	}
	return &GFPoly{field, coefficients}
}
