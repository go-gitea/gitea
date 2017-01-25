package query

import __yyfmt__ "fmt"

//line query_string.y:2
import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func logDebugGrammar(format string, v ...interface{}) {
	if debugParser {
		logger.Printf(format, v...)
	}
}

//line query_string.y:17
type yySymType struct {
	yys int
	s   string
	n   int
	f   float64
	q   Query
	pf  *float64
}

const tSTRING = 57346
const tPHRASE = 57347
const tPLUS = 57348
const tMINUS = 57349
const tCOLON = 57350
const tBOOST = 57351
const tNUMBER = 57352
const tGREATER = 57353
const tLESS = 57354
const tEQUAL = 57355
const tTILDE = 57356

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"tSTRING",
	"tPHRASE",
	"tPLUS",
	"tMINUS",
	"tCOLON",
	"tBOOST",
	"tNUMBER",
	"tGREATER",
	"tLESS",
	"tEQUAL",
	"tTILDE",
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line yacctab:1
var yyExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
	-1, 3,
	1, 3,
	-2, 5,
}

const yyNprod = 26
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 31

var yyAct = [...]int{

	16, 18, 21, 13, 27, 24, 17, 19, 20, 25,
	22, 15, 26, 23, 9, 11, 31, 14, 29, 3,
	10, 30, 2, 28, 5, 6, 7, 1, 4, 12,
	8,
}
var yyPact = [...]int{

	18, -1000, -1000, 18, 10, -1000, -1000, -1000, -6, 3,
	-1000, -1000, -1000, -1000, -1000, -4, -12, -1000, -1000, 0,
	-1, -1000, -1000, 13, -1000, -1000, 11, -1000, -1000, -1000,
	-1000, -1000,
}
var yyPgo = [...]int{

	0, 30, 29, 28, 27, 22, 19,
}
var yyR1 = [...]int{

	0, 4, 5, 5, 6, 3, 3, 3, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 2, 2,
}
var yyR2 = [...]int{

	0, 1, 2, 1, 3, 0, 1, 1, 1, 2,
	4, 1, 1, 3, 3, 3, 4, 5, 4, 5,
	4, 5, 4, 5, 0, 1,
}
var yyChk = [...]int{

	-1000, -4, -5, -6, -3, 6, 7, -5, -1, 4,
	10, 5, -2, 9, 14, 8, 4, 10, 5, 11,
	12, 14, 10, 13, 5, 10, 13, 5, 10, 5,
	10, 5,
}
var yyDef = [...]int{

	5, -2, 1, -2, 0, 6, 7, 2, 24, 8,
	11, 12, 4, 25, 9, 0, 13, 14, 15, 0,
	0, 10, 16, 0, 20, 18, 0, 22, 17, 21,
	19, 23,
}
var yyTok1 = [...]int{

	1,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14,
}
var yyTok3 = [...]int{
	0,
}

var yyErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

//line yaccpar:1

/*	parser for yacc output	*/

var (
	yyDebug        = 0
	yyErrorVerbose = false
)

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyParser interface {
	Parse(yyLexer) int
	Lookahead() int
}

type yyParserImpl struct {
	lval  yySymType
	stack [yyInitialStackSize]yySymType
	char  int
}

func (p *yyParserImpl) Lookahead() int {
	return p.char
}

func yyNewParser() yyParser {
	return &yyParserImpl{}
}

const yyFlag = -1000

func yyTokname(c int) string {
	if c >= 1 && c-1 < len(yyToknames) {
		if yyToknames[c-1] != "" {
			return yyToknames[c-1]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yyErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !yyErrorVerbose {
		return "syntax error"
	}

	for _, e := range yyErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + yyTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := yyPact[state]
	for tok := TOKSTART; tok-1 < len(yyToknames); tok++ {
		if n := base + tok; n >= 0 && n < yyLast && yyChk[yyAct[n]] == tok {
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}
	}

	if yyDef[state] == -2 {
		i := 0
		for yyExca[i] != -1 || yyExca[i+1] != state {
			i += 2
		}

		// Look for tokens that we accept or reduce.
		for i += 2; yyExca[i] >= 0; i += 2 {
			tok := yyExca[i]
			if tok < TOKSTART || yyExca[i+1] == 0 {
				continue
			}
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if yyExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += yyTokname(tok)
	}
	return res
}

func yylex1(lex yyLexer, lval *yySymType) (char, token int) {
	token = 0
	char = lex.Lex(lval)
	if char <= 0 {
		token = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		token = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			token = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		token = yyTok3[i+0]
		if token == char {
			token = yyTok3[i+1]
			goto out
		}
	}

out:
	if token == 0 {
		token = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(token), uint(char))
	}
	return char, token
}

func yyParse(yylex yyLexer) int {
	return yyNewParser().Parse(yylex)
}

func (yyrcvr *yyParserImpl) Parse(yylex yyLexer) int {
	var yyn int
	var yyVAL yySymType
	var yyDollar []yySymType
	_ = yyDollar // silence set and not used
	yyS := yyrcvr.stack[:]

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yyrcvr.char = -1
	yytoken := -1 // yyrcvr.char translated into internal numbering
	defer func() {
		// Make sure we report no lookahead when not parsing.
		yystate = -1
		yyrcvr.char = -1
		yytoken = -1
	}()
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yytoken), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yyrcvr.char < 0 {
		yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
	}
	yyn += yytoken
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yytoken { /* valid shift */
		yyrcvr.char = -1
		yytoken = -1
		yyVAL = yyrcvr.lval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yyrcvr.char < 0 {
			yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yytoken {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error(yyErrorMessage(yystate, yytoken))
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yytoken))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yytoken))
			}
			if yytoken == yyEofCode {
				goto ret1
			}
			yyrcvr.char = -1
			yytoken = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= yyR2[yyn]
	// yyp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
	if yyp+1 >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 1:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:39
		{
			logDebugGrammar("INPUT")
		}
	case 2:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line query_string.y:44
		{
			logDebugGrammar("SEARCH PARTS")
		}
	case 3:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:48
		{
			logDebugGrammar("SEARCH PART")
		}
	case 4:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line query_string.y:53
		{
			query := yyDollar[2].q
			if yyDollar[3].pf != nil {
				if query, ok := query.(BoostableQuery); ok {
					query.SetBoost(*yyDollar[3].pf)
				}
			}
			switch yyDollar[1].n {
			case queryShould:
				yylex.(*lexerWrapper).query.AddShould(query)
			case queryMust:
				yylex.(*lexerWrapper).query.AddMust(query)
			case queryMustNot:
				yylex.(*lexerWrapper).query.AddMustNot(query)
			}
		}
	case 5:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line query_string.y:72
		{
			yyVAL.n = queryShould
		}
	case 6:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:76
		{
			logDebugGrammar("PLUS")
			yyVAL.n = queryMust
		}
	case 7:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:81
		{
			logDebugGrammar("MINUS")
			yyVAL.n = queryMustNot
		}
	case 8:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:87
		{
			str := yyDollar[1].s
			logDebugGrammar("STRING - %s", str)
			var q FieldableQuery
			if strings.HasPrefix(str, "/") && strings.HasSuffix(str, "/") {
				q = NewRegexpQuery(str[1 : len(str)-1])
			} else if strings.ContainsAny(str, "*?") {
				q = NewWildcardQuery(str)
			} else {
				q = NewMatchQuery(str)
			}
			yyVAL.q = q
		}
	case 9:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line query_string.y:101
		{
			str := yyDollar[1].s
			fuzziness, err := strconv.ParseFloat(yyDollar[2].s, 64)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid fuzziness value: %v", err))
			}
			logDebugGrammar("FUZZY STRING - %s %f", str, fuzziness)
			q := NewMatchQuery(str)
			q.SetFuzziness(int(fuzziness))
			yyVAL.q = q
		}
	case 10:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line query_string.y:113
		{
			field := yyDollar[1].s
			str := yyDollar[3].s
			fuzziness, err := strconv.ParseFloat(yyDollar[4].s, 64)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid fuzziness value: %v", err))
			}
			logDebugGrammar("FIELD - %s FUZZY STRING - %s %f", field, str, fuzziness)
			q := NewMatchQuery(str)
			q.SetFuzziness(int(fuzziness))
			q.SetField(field)
			yyVAL.q = q
		}
	case 11:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:127
		{
			str := yyDollar[1].s
			logDebugGrammar("STRING - %s", str)
			q := NewMatchQuery(str)
			yyVAL.q = q
		}
	case 12:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:134
		{
			phrase := yyDollar[1].s
			logDebugGrammar("PHRASE - %s", phrase)
			q := NewMatchPhraseQuery(phrase)
			yyVAL.q = q
		}
	case 13:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line query_string.y:141
		{
			field := yyDollar[1].s
			str := yyDollar[3].s
			logDebugGrammar("FIELD - %s STRING - %s", field, str)
			var q FieldableQuery
			if strings.HasPrefix(str, "/") && strings.HasSuffix(str, "/") {
				q = NewRegexpQuery(str[1 : len(str)-1])
			} else if strings.ContainsAny(str, "*?") {
				q = NewWildcardQuery(str)
			} else {
				q = NewMatchQuery(str)
			}
			q.SetField(field)
			yyVAL.q = q
		}
	case 14:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line query_string.y:157
		{
			field := yyDollar[1].s
			str := yyDollar[3].s
			logDebugGrammar("FIELD - %s STRING - %s", field, str)
			q := NewMatchQuery(str)
			q.SetField(field)
			yyVAL.q = q
		}
	case 15:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line query_string.y:166
		{
			field := yyDollar[1].s
			phrase := yyDollar[3].s
			logDebugGrammar("FIELD - %s PHRASE - %s", field, phrase)
			q := NewMatchPhraseQuery(phrase)
			q.SetField(field)
			yyVAL.q = q
		}
	case 16:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line query_string.y:175
		{
			field := yyDollar[1].s
			min, _ := strconv.ParseFloat(yyDollar[4].s, 64)
			minInclusive := false
			logDebugGrammar("FIELD - GREATER THAN %f", min)
			q := NewNumericRangeInclusiveQuery(&min, nil, &minInclusive, nil)
			q.SetField(field)
			yyVAL.q = q
		}
	case 17:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line query_string.y:185
		{
			field := yyDollar[1].s
			min, _ := strconv.ParseFloat(yyDollar[5].s, 64)
			minInclusive := true
			logDebugGrammar("FIELD - GREATER THAN OR EQUAL %f", min)
			q := NewNumericRangeInclusiveQuery(&min, nil, &minInclusive, nil)
			q.SetField(field)
			yyVAL.q = q
		}
	case 18:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line query_string.y:195
		{
			field := yyDollar[1].s
			max, _ := strconv.ParseFloat(yyDollar[4].s, 64)
			maxInclusive := false
			logDebugGrammar("FIELD - LESS THAN %f", max)
			q := NewNumericRangeInclusiveQuery(nil, &max, nil, &maxInclusive)
			q.SetField(field)
			yyVAL.q = q
		}
	case 19:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line query_string.y:205
		{
			field := yyDollar[1].s
			max, _ := strconv.ParseFloat(yyDollar[5].s, 64)
			maxInclusive := true
			logDebugGrammar("FIELD - LESS THAN OR EQUAL %f", max)
			q := NewNumericRangeInclusiveQuery(nil, &max, nil, &maxInclusive)
			q.SetField(field)
			yyVAL.q = q
		}
	case 20:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line query_string.y:215
		{
			field := yyDollar[1].s
			minInclusive := false
			phrase := yyDollar[4].s

			logDebugGrammar("FIELD - GREATER THAN DATE %s", phrase)
			minTime, err := queryTimeFromString(phrase)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid time: %v", err))
			}
			q := NewDateRangeInclusiveQuery(minTime, time.Time{}, &minInclusive, nil)
			q.SetField(field)
			yyVAL.q = q
		}
	case 21:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line query_string.y:230
		{
			field := yyDollar[1].s
			minInclusive := true
			phrase := yyDollar[5].s

			logDebugGrammar("FIELD - GREATER THAN OR EQUAL DATE %s", phrase)
			minTime, err := queryTimeFromString(phrase)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid time: %v", err))
			}
			q := NewDateRangeInclusiveQuery(minTime, time.Time{}, &minInclusive, nil)
			q.SetField(field)
			yyVAL.q = q
		}
	case 22:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line query_string.y:245
		{
			field := yyDollar[1].s
			maxInclusive := false
			phrase := yyDollar[4].s

			logDebugGrammar("FIELD - LESS THAN DATE %s", phrase)
			maxTime, err := queryTimeFromString(phrase)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid time: %v", err))
			}
			q := NewDateRangeInclusiveQuery(time.Time{}, maxTime, nil, &maxInclusive)
			q.SetField(field)
			yyVAL.q = q
		}
	case 23:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line query_string.y:260
		{
			field := yyDollar[1].s
			maxInclusive := true
			phrase := yyDollar[5].s

			logDebugGrammar("FIELD - LESS THAN OR EQUAL DATE %s", phrase)
			maxTime, err := queryTimeFromString(phrase)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid time: %v", err))
			}
			q := NewDateRangeInclusiveQuery(time.Time{}, maxTime, nil, &maxInclusive)
			q.SetField(field)
			yyVAL.q = q
		}
	case 24:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line query_string.y:276
		{
			yyVAL.pf = nil
		}
	case 25:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line query_string.y:280
		{
			yyVAL.pf = nil
			boost, err := strconv.ParseFloat(yyDollar[1].s, 64)
			if err != nil {
				yylex.(*lexerWrapper).lex.Error(fmt.Sprintf("invalid boost value: %v", err))
			} else {
				yyVAL.pf = &boost
			}
			logDebugGrammar("BOOST %f", boost)
		}
	}
	goto yystack /* stack new state and value */
}
