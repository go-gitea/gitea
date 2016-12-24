%{
// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/charset"
	"github.com/pingcap/tidb/util/stringutil"
)

type lexer struct {
	c		int
	col		int
	errs		[]error
	expr		ast.ExprNode
	i		int
	inj		int
	lcol		int
	line		int
	list		[]ast.StmtNode
	ncol		int
	nline		int
	sc		int
	src		string
	val		[]byte
	ungetBuf	[]byte
	root		bool
	prepare		bool
	stmtStartPos 	int
	stringLit 	[]byte

	// record token's offset of the input
	tokenEndOffset   int
	tokenStartOffset int

	// Charset information
	charset		string
	collation	string
}


// NewLexer builds a new lexer.
func NewLexer(src string) (l *lexer) {
	l = &lexer{
		src:	src,
		nline:	1,
		ncol:	0,
	}
	l.next()
	return
}

func (l *lexer) Errors() []error {
	return l.errs
}

func (l *lexer) Stmts() []ast.StmtNode {
	return l.list
}

func (l *lexer) Expr() ast.ExprNode {
	return l.expr
}

func (l *lexer) Inj() int {
	return l.inj
}

func (l *lexer) SetInj(inj int) {
	l.inj = inj
}

func (l *lexer) SetPrepare() {
	l.prepare = true
}

func (l *lexer) IsPrepare() bool {
	return l.prepare
}

func (l *lexer) Root() bool {
	return l.root
}

func (l *lexer) SetRoot(root bool) {
	l.root = root
}

func (l *lexer) SetCharsetInfo(charset, collation string) {
	l.charset = charset
	l.collation = collation
}

func (l *lexer) GetCharsetInfo() (string, string) {
	return l.charset, l.collation
}

// The select statement is not at the end of the whole statement, if the last
// field text was set from its offset to the end of the src string, update
// the last field text.
func (l *lexer) SetLastSelectFieldText(st *ast.SelectStmt, lastEnd int) {
	lastField := st.Fields.Fields[len(st.Fields.Fields)-1]
	if lastField.Offset + len(lastField.Text()) >= len(l.src)-1 {
		lastField.SetText(l.src[lastField.Offset:lastEnd])
	}
}

func (l *lexer) startOffset(offset int) int {
	offset--
	for unicode.IsSpace(rune(l.src[offset])) {
		offset++
	}
	return offset
}

func (l *lexer) endOffset(offset int) int {
	offset--
	for offset > 0 && unicode.IsSpace(rune(l.src[offset-1])) {
		offset--
	}
	return offset
}

func (l *lexer) unget(b byte) {
	l.ungetBuf = append(l.ungetBuf, b)
	l.i--
	l.ncol--
	l.tokenEndOffset--
}

func (l *lexer) next() int {
	if un := len(l.ungetBuf); un > 0 {
		nc := l.ungetBuf[0]
		l.ungetBuf = l.ungetBuf[1:]
		l.c = int(nc)
		return  l.c
	}

	if l.c != 0 {
		l.val = append(l.val, byte(l.c))
	}
	l.c = 0
	if l.i < len(l.src) {
		l.c = int(l.src[l.i])
		l.i++
	}
	switch l.c {
	case '\n':
		l.lcol = l.ncol
		l.nline++
		l.ncol = 0
	default:
		l.ncol++
	}
	l.tokenEndOffset++
	return l.c
}

func (l *lexer) err0(ln, c int, arg interface{}) {
	var argStr string
	if arg != nil {
		argStr = fmt.Sprintf(" %v", arg)
	}

	err := fmt.Errorf("line %d column %d near \"%s\"%s", ln, c, l.val, argStr)
	l.errs = append(l.errs, err)
}

func (l *lexer) err(arg interface{}) {
	l.err0(l.line, l.col, arg)
}

func (l *lexer) errf(format string, args ...interface{}) {
	s := fmt.Sprintf(format, args...)
	l.err0(l.line, l.col, s)
}

func (l *lexer) Error(s string) {
	// Notice: ignore origin error info.
	l.err(nil)
}

func (l *lexer) stmtText() string {
	endPos := l.i
	if l.src[l.i-1] == '\n' {
		endPos = l.i-1 // trim new line	
	}
	if l.src[l.stmtStartPos] == '\n' {
		l.stmtStartPos++	
	}

	text := l.src[l.stmtStartPos:endPos] 

	l.stmtStartPos = l.i
	return text
}


func (l *lexer) Lex(lval *yySymType) (r int) {
	defer func() {
		lval.line, lval.col, lval.offset = l.line, l.col, l.tokenStartOffset
		l.tokenStartOffset = l.tokenEndOffset
	}()
	const (
		INITIAL = iota
		S1
		S2
		S3
		S4
	)

	if n := l.inj; n != 0 {
		l.inj = 0
		return n
	}

	c0, c := 0, l.c
%}

int_lit		{decimal_lit}|{octal_lit}
decimal_lit	[1-9][0-9]*
octal_lit	0[0-7]*
hex_lit		0[xX][0-9a-fA-F]+|[xX]"'"[0-9a-fA-F]+"'"
bit_lit 	0[bB][01]+|[bB]"'"[01]+"'"

float_lit	{D}"."{D}?{E}?|{D}{E}|"."{D}{E}?
D		[0-9]+
E		[eE][-+]?[0-9]+

imaginary_ilit	{D}i
imaginary_lit	{float_lit}i

a		[aA]
b		[bB]
c		[cC]
d		[dD]
e		[eE]
f		[fF]
g		[gG]
h		[hH]
i		[iI]
j		[jJ]
k		[kK]
l		[lL]
m		[mM]
n		[nN]
o		[oO]
p		[pP]
q		[qQ]
r		[rR]
s		[sS]
t		[tT]
u		[uU]
v		[vV]
w		[wW]
x		[xX]
y		[yY]
z		[zZ]

abs		{a}{b}{s}
add		{a}{d}{d}
adddate		{a}{d}{d}{d}{a}{t}{e}
admin		{a}{d}{m}{i}{n}
after		{a}{f}{t}{e}{r}
all		{a}{l}{l}
alter		{a}{l}{t}{e}{r}
and		{a}{n}{d}
any 		{a}{n}{y}
as		{a}{s}
asc		{a}{s}{c}
auto_increment	{a}{u}{t}{o}_{i}{n}{c}{r}{e}{m}{e}{n}{t}
avg		{a}{v}{g}
avg_row_length	{a}{v}{g}_{r}{o}{w}_{l}{e}{n}{g}{t}{h}
begin		{b}{e}{g}{i}{n}
between		{b}{e}{t}{w}{e}{e}{n}
both		{b}{o}{t}{h}
btree		{b}{t}{r}{e}{e}
by		{b}{y}
case		{c}{a}{s}{e}
cast		{c}{a}{s}{t}
character	{c}{h}{a}{r}{a}{c}{t}{e}{r}
charset		{c}{h}{a}{r}{s}{e}{t}
check 		{c}{h}{e}{c}{k}
checksum 	{c}{h}{e}{c}{k}{s}{u}{m}
coalesce	{c}{o}{a}{l}{e}{s}{c}{e}
collate		{c}{o}{l}{l}{a}{t}{e}
collation	{c}{o}{l}{l}{a}{t}{i}{o}{n}
column		{c}{o}{l}{u}{m}{n}
columns		{c}{o}{l}{u}{m}{n}{s}
comment 	{c}{o}{m}{m}{e}{n}{t}
commit		{c}{o}{m}{m}{i}{t}
committed	{c}{o}{m}{m}{i}{t}{t}{e}{d}
compact		{c}{o}{m}{p}{a}{c}{t}
compressed	{c}{o}{m}{p}{r}{e}{s}{s}{e}{d}
compression	{c}{o}{m}{p}{r}{e}{s}{s}{i}{o}{n}
concat		{c}{o}{n}{c}{a}{t}
concat_ws	{c}{o}{n}{c}{a}{t}_{w}{s}
connection	{c}{o}{n}{n}{e}{c}{t}{i}{o}{n}
connection_id	{c}{o}{n}{n}{e}{c}{t}{i}{o}{n}_{i}{d}
constraint	{c}{o}{n}{s}{t}{r}{a}{i}{n}{t}
convert		{c}{o}{n}{v}{e}{r}{t}
count		{c}{o}{u}{n}{t}
create		{c}{r}{e}{a}{t}{e}
cross		{c}{r}{o}{s}{s}
curdate 	{c}{u}{r}{d}{a}{t}{e}
current_date	{c}{u}{r}{r}{e}{n}{t}_{d}{a}{t}{e}
curtime 	{c}{u}{r}{t}{i}{m}{e}
current_time	{c}{u}{r}{r}{e}{n}{t}_{t}{i}{m}{e}
current_user	{c}{u}{r}{r}{e}{n}{t}_{u}{s}{e}{r}
database	{d}{a}{t}{a}{b}{a}{s}{e}
databases	{d}{a}{t}{a}{b}{a}{s}{e}{s}
date_add	{d}{a}{t}{e}_{a}{d}{d}
date_sub	{d}{a}{t}{e}_{s}{u}{b}
day		{d}{a}{y}
dayname	{d}{a}{y}{n}{a}{m}{e}
dayofweek	{d}{a}{y}{o}{f}{w}{e}{e}{k}
dayofmonth	{d}{a}{y}{o}{f}{m}{o}{n}{t}{h}
dayofyear	{d}{a}{y}{o}{f}{y}{e}{a}{r}
ddl		{d}{d}{l}
deallocate	{d}{e}{a}{l}{l}{o}{c}{a}{t}{e}
default		{d}{e}{f}{a}{u}{l}{t}
delayed		{d}{e}{l}{a}{y}{e}{d}
delay_key_write	{d}{e}{l}{a}{y}_{k}{e}{y}_{w}{r}{i}{t}{e}
delete		{d}{e}{l}{e}{t}{e}
drop		{d}{r}{o}{p}
desc		{d}{e}{s}{c}
describe	{d}{e}{s}{c}{r}{i}{b}{e}
distinct	{d}{i}{s}{t}{i}{n}{c}{t}
div		{d}{i}{v}
do		{d}{o}
dual 		{d}{u}{a}{l}
duplicate	{d}{u}{p}{l}{i}{c}{a}{t}{e}
dynamic		{d}{y}{n}{a}{m}{i}{c}
else		{e}{l}{s}{e}
end		{e}{n}{d}
engine		{e}{n}{g}{i}{n}{e}
engines		{e}{n}{g}{i}{n}{e}{s}
escape		{e}{s}{c}{a}{p}{e}
execute		{e}{x}{e}{c}{u}{t}{e}
exists		{e}{x}{i}{s}{t}{s}
explain		{e}{x}{p}{l}{a}{i}{n}
extract		{e}{x}{t}{r}{a}{c}{t}
fields		{f}{i}{e}{l}{d}{s}
first		{f}{i}{r}{s}{t}
fixed		{f}{i}{x}{e}{d}
for		{f}{o}{r}
foreign		{f}{o}{r}{e}{i}{g}{n}
found_rows	{f}{o}{u}{n}{d}_{r}{o}{w}{s}
from		{f}{r}{o}{m}
full		{f}{u}{l}{l}
fulltext	{f}{u}{l}{l}{t}{e}{x}{t}
global		{g}{l}{o}{b}{a}{l}
grant		{g}{r}{a}{n}{t}
grants		{g}{r}{a}{n}{t}{s}
group		{g}{r}{o}{u}{p}
group_concat	{g}{r}{o}{u}{p}_{c}{o}{n}{c}{a}{t}
hash		{h}{a}{s}{h}
having		{h}{a}{v}{i}{n}{g}
high_priority	{h}{i}{g}{h}_{p}{r}{i}{o}{r}{i}{t}{y}
hour		{h}{o}{u}{r}
identified	{i}{d}{e}{n}{t}{i}{f}{i}{e}{d}
if		{i}{f}
ifnull		{i}{f}{n}{u}{l}{l}
ignore		{i}{g}{n}{o}{r}{e}
in		{i}{n}
index		{i}{n}{d}{e}{x}
inner 		{i}{n}{n}{e}{r}
insert		{i}{n}{s}{e}{r}{t}
interval	{i}{n}{t}{e}{r}{v}{a}{l}
into		{i}{n}{t}{o}
is		{i}{s}
isolation	{i}{s}{o}{l}{a}{t}{i}{o}{n}
join		{j}{o}{i}{n}
key		{k}{e}{y}
key_block_size	{k}{e}{y}_{b}{l}{o}{c}{k}_{s}{i}{z}{e}
leading		{l}{e}{a}{d}{i}{n}{g}
left		{l}{e}{f}{t}
length		{l}{e}{n}{g}{t}{h}
level		{l}{e}{v}{e}{l}
like		{l}{i}{k}{e}
limit		{l}{i}{m}{i}{t}
local		{l}{o}{c}{a}{l}
locate		{l}{o}{c}{a}{t}{e}
lock		{l}{o}{c}{k}
lower		{l}{o}{w}{e}{r}
low_priority	{l}{o}{w}_{p}{r}{i}{o}{r}{i}{t}{y}
max_rows	{m}{a}{x}_{r}{o}{w}{s}
microsecond	{m}{i}{c}{r}{o}{s}{e}{c}{o}{n}{d}
minute		{m}{i}{n}{u}{t}{e}
min_rows	{m}{i}{n}_{r}{o}{w}{s}
mod 		{m}{o}{d}
mode		{m}{o}{d}{e}
month		{m}{o}{n}{t}{h}
names		{n}{a}{m}{e}{s}
national	{n}{a}{t}{i}{o}{n}{a}{l}
not		{n}{o}{t}
offset		{o}{f}{f}{s}{e}{t}
on		{o}{n}
only		{o}{n}{l}{y}
option		{o}{p}{t}{i}{o}{n}
or		{o}{r}
order		{o}{r}{d}{e}{r}
outer		{o}{u}{t}{e}{r}
password	{p}{a}{s}{s}{w}{o}{r}{d}
pow 		{p}{o}{w}
power		{p}{o}{w}{e}{r}
prepare		{p}{r}{e}{p}{a}{r}{e}
primary		{p}{r}{i}{m}{a}{r}{y}
procedure	{p}{r}{o}{c}{e}{d}{u}{r}{e}
quarter		{q}{u}{a}{r}{t}{e}{r}
quick		{q}{u}{i}{c}{k}
rand		{r}{a}{n}{d}
read		{r}{e}{a}{d}
repeat		{r}{e}{p}{e}{a}{t}
repeatable	{r}{e}{p}{e}{a}{t}{a}{b}{l}{e}
references	{r}{e}{f}{e}{r}{e}{n}{c}{e}{s}
regexp		{r}{e}{g}{e}{x}{p}
replace		{r}{e}{p}{l}{a}{c}{e}
redundant	{r}{e}{d}{u}{n}{d}{a}{n}{t}
right		{r}{i}{g}{h}{t}
rlike		{r}{l}{i}{k}{e}
rollback	{r}{o}{l}{l}{b}{a}{c}{k}
row 		{r}{o}{w}
row_format	{r}{o}{w}_{f}{o}{r}{m}{a}{t}
schema		{s}{c}{h}{e}{m}{a}
schemas		{s}{c}{h}{e}{m}{a}{s}
second		{s}{e}{c}{o}{n}{d}
select		{s}{e}{l}{e}{c}{t}
serializable	{s}{e}{r}{i}{a}{l}{i}{z}{a}{b}{l}{e}
session		{s}{e}{s}{s}{i}{o}{n}
set		{s}{e}{t}
share		{s}{h}{a}{r}{e}
show		{s}{h}{o}{w}
some		{s}{o}{m}{e}
start		{s}{t}{a}{r}{t}
status          {s}{t}{a}{t}{u}{s}
subdate		{s}{u}{b}{d}{a}{t}{e}
strcmp		{s}{t}{r}{c}{m}{p}
substr		{s}{u}{b}{s}{t}{r}
substring	{s}{u}{b}{s}{t}{r}{i}{n}{g}
substring_index	{s}{u}{b}{s}{t}{r}{i}{n}{g}_{i}{n}{d}{e}{x}
sum		{s}{u}{m}
sysdate		{s}{y}{s}{d}{a}{t}{e}
table		{t}{a}{b}{l}{e}
tables		{t}{a}{b}{l}{e}{s}
then		{t}{h}{e}{n}
to		{t}{o}
trailing	{t}{r}{a}{i}{l}{i}{n}{g}
transaction	{t}{r}{a}{n}{s}{a}{c}{t}{i}{o}{n}
triggers	{t}{r}{i}{g}{g}{e}{r}{s}
trim		{t}{r}{i}{m}
truncate	{t}{r}{u}{n}{c}{a}{t}{e}
max		{m}{a}{x}
min		{m}{i}{n}
uncommitted	{u}{n}{c}{o}{m}{m}{i}{t}{t}{e}{d}
unknown		{u}{n}{k}{n}{o}{w}{n}
union		{u}{n}{i}{o}{n}
unique		{u}{n}{i}{q}{u}{e}
unlock		{u}{n}{l}{o}{c}{k}
nullif		{n}{u}{l}{l}{i}{f}
update		{u}{p}{d}{a}{t}{e}
upper		{u}{p}{p}{e}{r}
value		{v}{a}{l}{u}{e}
values		{v}{a}{l}{u}{e}{s}
variables	{v}{a}{r}{i}{a}{b}{l}{e}{s}
version		{v}{e}{r}{s}{i}{o}{n}
warnings	{w}{a}{r}{n}{i}{n}{g}{s}
week		{w}{e}{e}{k}
weekday		{w}{e}{e}{k}{d}{a}{y}
weekofyear	{w}{e}{e}{k}{o}{f}{y}{e}{a}{r}
where		{w}{h}{e}{r}{e}
when		{w}{h}{e}{n}
write		{w}{r}{i}{t}{e}
xor		{x}{o}{r}
yearweek	{y}{e}{a}{r}{w}{e}{e}{k}

null		{n}{u}{l}{l}
false		{f}{a}{l}{s}{e}
true		{t}{r}{u}{e}

calc_found_rows	{s}{q}{l}_{c}{a}{l}{c}_{f}{o}{u}{n}{d}_{r}{o}{w}{s}

current_ts	{c}{u}{r}{r}{e}{n}{t}_{t}{i}{m}{e}{s}{t}{a}{m}{p}
localtime	{l}{o}{c}{a}{l}{t}{i}{m}{e}
localts		{l}{o}{c}{a}{l}{t}{i}{m}{e}{s}{t}{a}{m}{p}
now		{n}{o}{w}

bit		{b}{i}{t}
tiny		{t}{i}{n}{y}
tinyint		{t}{i}{n}{y}{i}{n}{t}
smallint	{s}{m}{a}{l}{l}{i}{n}{t}
mediumint	{m}{e}{d}{i}{u}{m}{i}{n}{t}
int		{i}{n}{t}
integer		{i}{n}{t}{e}{g}{e}{r}
bigint		{b}{i}{g}{i}{n}{t}
real		{r}{e}{a}{l}
double		{d}{o}{u}{b}{l}{e}
float		{f}{l}{o}{a}{t}
decimal		{d}{e}{c}{i}{m}{a}{l}
numeric		{n}{u}{m}{e}{r}{i}{c}
date		{d}{a}{t}{e}
time		{t}{i}{m}{e}
timestamp	{t}{i}{m}{e}{s}{t}{a}{m}{p}
datetime	{d}{a}{t}{e}{t}{i}{m}{e}
year		{y}{e}{a}{r}
char		{c}{h}{a}{r}
varchar		{v}{a}{r}{c}{h}{a}{r}
binary		{b}{i}{n}{a}{r}{y}
varbinary	{v}{a}{r}{b}{i}{n}{a}{r}{y}
tinyblob	{t}{i}{n}{y}{b}{l}{o}{b}
blob		{b}{l}{o}{b}
mediumblob	{m}{e}{d}{i}{u}{m}{b}{l}{o}{b}
longblob	{l}{o}{n}{g}{b}{l}{o}{b}
tinytext	{t}{i}{n}{y}{t}{e}{x}{t}
text		{t}{e}{x}{t}
mediumtext	{m}{e}{d}{i}{u}{m}{t}{e}{x}{t}
longtext	{l}{o}{n}{g}{t}{e}{x}{t}
enum		{e}{n}{u}{m}
precision	{p}{r}{e}{c}{i}{s}{i}{o}{n}

signed		{s}{i}{g}{n}{e}{d}
unsigned	{u}{n}{s}{i}{g}{n}{e}{d}
zerofill	{z}{e}{r}{o}{f}{i}{l}{l}

bigrat		{b}{i}{g}{r}{a}{t}
bool		{b}{o}{o}{l}
boolean		{b}{o}{o}{l}{e}{a}{n}
byte		{b}{y}{t}{e}
duration	{d}{u}{r}{a}{t}{i}{o}{n}
rune		{r}{u}{n}{e}
string		{s}{t}{r}{i}{n}{g}
use		{u}{s}{e}
user		{u}{s}{e}{r}
using		{u}{s}{i}{n}{g}

idchar0		[a-zA-Z_]
idchars		{idchar0}|[0-9$] // See: https://dev.mysql.com/doc/refman/5.7/en/identifiers.html
ident		{idchar0}{idchars}*

user_var	"@"{ident}
sys_var		"@@"(({global}".")|({session}".")|{local}".")?{ident}

second_microsecond	{s}{e}{c}{o}{n}{d}_{m}{i}{c}{r}{o}{s}{e}{c}{o}{n}{d}
minute_microsecond	{m}{i}{n}{u}{t}{e}_{m}{i}{c}{r}{o}{s}{e}{c}{o}{n}{d}
minute_second 		{m}{i}{n}{u}{t}{e}_{s}{e}{c}{o}{n}{d}
hour_microsecond	{h}{o}{u}{r}_{m}{i}{c}{r}{o}{s}{e}{c}{o}{n}{d}
hour_second 		{h}{o}{u}{r}_{s}{e}{c}{o}{n}{d}
hour_minute		{h}{o}{u}{r}_{m}{i}{n}{u}{t}{e}
day_microsecond		{d}{a}{y}_{m}{i}{c}{r}{o}{s}{e}{c}{o}{n}{d}
day_second 		{d}{a}{y}_{s}{e}{c}{o}{n}{d}
day_minute		{d}{a}{y}_{m}{i}{n}{u}{t}{e}
day_hour		{d}{a}{y}_{h}{o}{u}{r}
year_month		{y}{e}{a}{r}_{m}{o}{n}{t}{h}

%yyc c
%yyn c = l.next()
%yyt l.sc

%x S1 S2 S3 S4

%%
		l.val = l.val[:0]
		c0, l.line, l.col = l.c, l.nline, l.ncol
                        
<*>\0		return 0

[ \t\n\r]+
#.*
\/\/.*
\/\*([^*]|\*+[^*/])*\*+\/
--			l.sc = S3
<S3>[ \t]+.*		{l.sc = 0} 
<S3>[^ \t]		{
				l.sc = 0
				l.c = '-'
				n := len(l.val)
				l.unget(l.val[n-1])
				return '-' 
			}

{int_lit}		return l.int(lval)
{float_lit}		return l.float(lval)
{hex_lit}		return l.hex(lval)
{bit_lit}		return l.bit(lval)

\"			l.sc = S1
'			l.sc = S2
`			l.sc = S4

<S1>[^\"\\]*		l.stringLit = append(l.stringLit, l.val...)	
<S1>\\.			l.stringLit = append(l.stringLit, l.val...)
<S1>\"\"		l.stringLit = append(l.stringLit, '"')
<S1>\"			l.stringLit = append(l.stringLit, '"')
			l.sc = 0
			return l.str(lval, "\"")	
<S2>[^'\\]*		l.stringLit = append(l.stringLit, l.val...)	
<S2>\\.			l.stringLit = append(l.stringLit, l.val...)
<S2>''			l.stringLit = append(l.stringLit, '\'')
<S2>'			l.stringLit = append(l.stringLit, '\'')
			l.sc = 0
			return l.str(lval, "'")
<S4>[^`]*		l.stringLit = append(l.stringLit, l.val...)	
<S4>``			l.stringLit = append(l.stringLit, '`')
<S4>`			l.sc = 0
			lval.item = string(l.stringLit)
			l.stringLit = l.stringLit[0:0]
			return identifier

"&&"			return andand
"&^"			return andnot
"<<"			return lsh
"<="			return le
"=" 			return eq
">="			return ge
"!="			return neq
"<>"			return neq
"||"			return oror
">>"			return rsh
"<=>"			return nulleq

"@"			return at
"?"			return placeholder

{abs}			lval.item = string(l.val)
			return abs
{add}			return add
{adddate}		lval.item = string(l.val)
			return addDate
{admin}			lval.item = string(l.val)
			return admin
{after}			lval.item = string(l.val)
			return after
{all}			return all
{alter}			return alter
{and}			return and
{any}			lval.item = string(l.val)
			return any
{asc}			return asc
{as}			return as
{auto_increment}	lval.item = string(l.val)
			return autoIncrement
{avg}			lval.item = string(l.val)
			return avg
{avg_row_length}	lval.item = string(l.val)
			return avgRowLength
{begin}			lval.item = string(l.val)
			return begin
{between}		return between
{both}			return both
{btree}			lval.item = string(l.val)
			return btree
{by}			return by
{case}			return caseKwd
{cast}			lval.item = string(l.val)
			return cast
{character}		return character
{charset}		lval.item = string(l.val)
			return charsetKwd
{check}			return check
{checksum}		lval.item = string(l.val)
			return checksum
{coalesce}		lval.item = string(l.val)
			return coalesce
{collate}		return collate
{collation}		lval.item = string(l.val)
			return collation
{column}		return column
{columns}		lval.item = string(l.val)
			return columns
{comment}		lval.item = string(l.val)
			return comment
{commit}		lval.item = string(l.val)
			return commit
{committed}		lval.item = string(l.val)
			return committed
{compact}		lval.item = string(l.val)
			return compact
{compressed}		lval.item = string(l.val)
			return compressed
{compression}		lval.item = string(l.val)
			return compression
{concat}		lval.item = string(l.val)
			return concat
{concat_ws}		lval.item = string(l.val)
			return concatWs
{connection}		lval.item = string(l.val)
			return connection
{connection_id}		lval.item = string(l.val)
			return connectionID
{constraint}		return constraint
{convert}		lval.item = string(l.val)
			return convert
{count}			lval.item = string(l.val)
			return count
{create}		return create
{cross}			return cross
{curdate}		lval.item = string(l.val)
			return curDate
{current_date}		lval.item = string(l.val)
			return currentDate
{curtime}		lval.item = string(l.val)
			return curTime
{current_time}		lval.item = string(l.val)
			return currentTime
{current_user}		lval.item = string(l.val)
			return currentUser
{database}		lval.item = string(l.val)
			return database
{databases}		return databases
{date_add}		lval.item = string(l.val)
			return dateAdd
{date_sub}		lval.item = string(l.val)
			return dateSub
{day}			lval.item = string(l.val)
			return day
{dayname}		lval.item = string(l.val)
			return dayname
{dayofweek}		lval.item = string(l.val)
			return dayofweek
{dayofmonth}		lval.item = string(l.val)
			return dayofmonth
{dayofyear}		lval.item = string(l.val)
			return dayofyear
{day_hour}		lval.item = string(l.val)
			return dayHour
{day_microsecond}	lval.item = string(l.val)
			return dayMicrosecond
{day_minute}		lval.item = string(l.val)
			return dayMinute
{day_second}		lval.item = string(l.val)
			return daySecond
{ddl}			return ddl
{deallocate}		lval.item = string(l.val)
			return deallocate
{default}		return defaultKwd
{delayed}		return delayed
{delay_key_write}	lval.item = string(l.val)
			return delayKeyWrite
{delete}		return deleteKwd
{desc}			return desc
{describe}		return describe
{drop}			return drop
{distinct}		return distinct
{div}			return div
{do}			lval.item = string(l.val)
			return do
{dual}			return dual
{duplicate}		lval.item = string(l.val)
			return duplicate
{dynamic}		lval.item = string(l.val)
			return dynamic
{else}			return elseKwd
{end}			lval.item = string(l.val)
			return end
{engine}		lval.item = string(l.val)
			return engine
{engines}		lval.item = string(l.val)
			return engines
{execute}		lval.item = string(l.val)
			return execute
{enum}			return enum
{escape}		lval.item = string(l.val)
			return escape
{exists}		return exists
{explain}		return explain
{extract}		lval.item = string(l.val)
			return extract
{fields}		lval.item = string(l.val)
			return fields
{first}			lval.item = string(l.val)
			return first
{fixed}			lval.item = string(l.val)
			return fixed
{for}			return forKwd
{foreign}		return foreign
{found_rows}		lval.item = string(l.val)
			return foundRows
{from}			return from
{full}			lval.item = string(l.val)
			return full
{fulltext}		return fulltext
{grant}			return grant
{grants}		lval.item = string(l.val)
			return grants
{group}			return group
{group_concat}		lval.item = string(l.val)
			return groupConcat
{hash}			lval.item = string(l.val)
			return hash
{having}		return having
{high_priority}		return highPriority
{hour}			lval.item = string(l.val)
			return hour
{hour_microsecond}	lval.item = string(l.val)
			return hourMicrosecond
{hour_minute}		lval.item = string(l.val)
			return hourMinute
{hour_second}		lval.item = string(l.val)
			return hourSecond
{identified}		lval.item = string(l.val)
			return identified
{if}			lval.item = string(l.val)
			return ifKwd
{ifnull}		lval.item = string(l.val)
			return ifNull
{ignore}		return ignore
{index}			return index
{inner} 		return inner
{insert}		return insert
{interval}		return interval
{into}			return into
{in}			return in
{is}			return is
{isolation}		lval.item = string(l.val)
			return isolation
{join}			return join
{key}			return key
{key_block_size}	lval.item = string(l.val)
			return keyBlockSize
{leading}		return leading
{left}			lval.item = string(l.val)
			return left
{length}		lval.item = string(l.val)
			return length
{level}			lval.item = string(l.val)
			return level
{like}			return like
{limit}			return limit
{local}			lval.item = string(l.val)
			return local
{locate}		lval.item = string(l.val)
			return locate
{lock}			return lock
{lower}			lval.item = string(l.val)
			return lower
{low_priority}		return lowPriority
{max}			lval.item = string(l.val)
			return max
{max_rows}		lval.item = string(l.val)
			return maxRows
{microsecond}		lval.item = string(l.val)
			return microsecond
{min}			lval.item = string(l.val)
			return min
{minute}		lval.item = string(l.val)
			return minute
{minute_microsecond}	lval.item = string(l.val)
			return minuteMicrosecond
{minute_second}		lval.item = string(l.val)
			return minuteSecond
{min_rows}		lval.item = string(l.val)
			return minRows
{mod}			return mod
{mode}			lval.item = string(l.val)
			return mode
{month}			lval.item = string(l.val)
			return month
{names}			lval.item = string(l.val)
			return names
{national}		lval.item = string(l.val)
			return national
{not}			return not
{offset}		lval.item = string(l.val)
			return offset
{on}			return on
{only}			lval.item = string(l.val)
			return only
{option}		return option
{order}			return order
{or}			return or
{outer}			return outer
{password}		lval.item = string(l.val)
			return password
{pow}			lval.item = string(l.val)
			return pow
{power}		lval.item = string(l.val)
			return power
{prepare}		lval.item = string(l.val)
			return prepare
{primary}		return primary
{procedure}		return procedure
{quarter}		lval.item = string(l.val)
			return quarter
{quick}			lval.item = string(l.val)
			return quick
redundant		lval.item = string(l.val)
			return redundant
{right}			return right
{rollback}		lval.item = string(l.val)
			return rollback
{row}			lval.item = string(l.val)
			return row
{row_format}		lval.item = string(l.val)
			return rowFormat
{schema}		lval.item = string(l.val)
			return schema
{schemas}		return schemas
{serializable}		lval.item = string(l.val)
			return serializable
{session}		lval.item = string(l.val)
			return session
{some}			lval.item = string(l.val)
			return some
{start}			lval.item = string(l.val)
			return start
{status}		lval.item = string(l.val)
			return status
{global}		lval.item = string(l.val)
			return global
{rand}			lval.item = string(l.val)
			return rand
{read}			return read
{repeat}		lval.item = string(l.val)
			return repeat
{repeatable}		lval.item = string(l.val)
			return repeatable
{regexp}		return regexpKwd
{replace}		lval.item = string(l.val)
			return replace
{references}		return references
{rlike}			return rlike

{sys_var}		lval.item = string(l.val)
			return sysVar

{user_var}		lval.item = string(l.val)
			return userVar
{second}		lval.item = string(l.val)
			return second
{second_microsecond}	lval.item= string(l.val)
			return secondMicrosecond
{select}		return selectKwd

{set}			return set
{share}			return share
{show}			return show
{subdate}		lval.item = string(l.val)
			return subDate
{strcmp}		lval.item = string(l.val)
			return strcmp
{substr}		lval.item = string(l.val)
			return substring
{substring}		lval.item = string(l.val)
			return substring
{substring_index}	lval.item = string(l.val)
			return substringIndex
{sum}			lval.item = string(l.val)
			return sum
{sysdate}		lval.item = string(l.val)
			return sysDate
{table}			return tableKwd
{tables}		lval.item = string(l.val)
			return tables
{then}			return then
{to}			return to
{trailing}		return trailing
{transaction}		lval.item = string(l.val)
			return transaction
{triggers}		lval.item = string(l.val)
			return triggers
{trim}			lval.item = string(l.val)
			return trim
{truncate}		lval.item = string(l.val)
			return truncate
{uncommitted}		lval.item = string(l.val)
			return uncommitted
{union}			return union
{unique}		return unique
{unknown}		lval.item = string(l.val)
			return unknown
{nullif}		lval.item = string(l.val)
			return nullIf
{unlock}		return unlock
{update}		return update
{upper}			lval.item = string(l.val)
			return upper
{use}			return use
{user}			lval.item = string(l.val)
			return user
{using}			return using
{value}			lval.item = string(l.val)
			return value
{values}		return values
{variables}		lval.item = string(l.val)
			return variables
{version}		lval.item = string(l.val)
			return version
{warnings}		lval.item = string(l.val)
			return warnings
{week}			lval.item = string(l.val)
			return week
{weekday}		lval.item = string(l.val)
			return weekday
{weekofyear}		lval.item = string(l.val)
			return weekofyear
{when}			return when
{where}			return where
{write}			return write
{xor}			return xor
{yearweek}		lval.item = string(l.val)
			return yearweek
{year_month}		lval.item = string(l.val)
			return yearMonth
			
{signed}		lval.item = string(l.val)
			return signed
{unsigned}		return unsigned
{zerofill}		return zerofill

{null}			lval.item = nil
			return null

{false}			return falseKwd

{true}			return trueKwd

{calc_found_rows}	lval.item = string(l.val)
			return calcFoundRows

{current_ts}		lval.item = string(l.val)
			return currentTs
{localtime}		return localTime
{localts}		return localTs
{now}			lval.item = string(l.val)
			return now

{bit}			lval.item = string(l.val) 
			return bitType

{tiny}			lval.item = string(l.val) 
			return tinyIntType

{tinyint}		lval.item = string(l.val) 
			return tinyIntType

{smallint}		lval.item = string(l.val) 
			return smallIntType

{mediumint}		lval.item = string(l.val)
			return mediumIntType

{bigint}		lval.item = string(l.val)
			return bigIntType

{decimal}		lval.item = string(l.val)
			return decimalType

{numeric}		lval.item = string(l.val)
			return numericType

{float}			lval.item = string(l.val)
			return floatType

{double}		lval.item = string(l.val)
			return doubleType

{precision}		lval.item = string(l.val)
			return precisionType

{real}			lval.item = string(l.val)
			return realType

{date}			lval.item = string(l.val)
			return dateType

{time}			lval.item = string(l.val) 
			return timeType

{timestamp}		lval.item = string(l.val)
			return timestampType

{datetime}		lval.item = string(l.val)
			return datetimeType

{year}			lval.item = string(l.val)
			return yearType

{char}			lval.item = string(l.val)
			return charType

{varchar}		lval.item = string(l.val)
			return varcharType

{binary}		lval.item = string(l.val)
			return binaryType

{varbinary}		lval.item = string(l.val)
			return varbinaryType

{tinyblob}		lval.item = string(l.val)
			return tinyblobType

{blob}			lval.item = string(l.val)
			return blobType

{mediumblob}		lval.item = string(l.val)
			return mediumblobType

{longblob}		lval.item = string(l.val)
			return longblobType

{tinytext}		lval.item = string(l.val)
			return tinytextType

{mediumtext}		lval.item = string(l.val)
			return mediumtextType

{text}			lval.item = string(l.val)
			return textType

{longtext}		lval.item = string(l.val)
			return longtextType

{bool}			lval.item = string(l.val) 
			return boolType

{boolean}		lval.item = string(l.val)
			return booleanType

{byte}			lval.item = string(l.val) 
			return byteType

{int}			lval.item = string(l.val)
			return intType

{integer}		lval.item = string(l.val)
			return integerType

{ident}			lval.item = string(l.val)
			return l.handleIdent(lval)

.			return c0

%%
			return int(unicode.ReplacementChar)
}

func (l *lexer) npos() (line, col int) {
	if line, col = l.nline, l.ncol; col == 0 {
		line--
		col = l.lcol+1
	}
	return
} 

func (l *lexer) str(lval *yySymType, pref string) int {
	l.sc = 0
	// TODO: performance issue.
	s := string(l.stringLit)
	l.stringLit = l.stringLit[0:0]
	if pref == "'" {
		s = strings.Replace(s, "\\'", "'", -1)    
		s = strings.TrimSuffix(s, "'") + "\""
		pref = "\""
	} 
	v := stringutil.RemoveUselessBackslash(pref+s)
	v, err := strconv.Unquote(v)
	if err != nil {
		v = strings.TrimSuffix(s, pref)
	}
	lval.item = v
	return stringLit
}

func (l *lexer) trimIdent(idt string) string {
	idt = strings.TrimPrefix(idt, "`")    
	idt = strings.TrimSuffix(idt, "`")    
	return idt
}

func (l *lexer) int(lval *yySymType) int {
	n, err := strconv.ParseUint(string(l.val), 0, 64)
	if err != nil {
		l.errf("integer literal: %v", err)
		return int(unicode.ReplacementChar)
	}

	switch {
	case n < math.MaxInt64:
		lval.item = int64(n)
	default:
		lval.item = uint64(n)
	}
	return intLit
}

func (l *lexer) float(lval *yySymType) int {
	n, err := strconv.ParseFloat(string(l.val), 64)
	if err != nil {
		l.errf("float literal: %v", err)
		return int(unicode.ReplacementChar)
	}

	lval.item = float64(n)
	return floatLit
}

// https://dev.mysql.com/doc/refman/5.7/en/hexadecimal-literals.html
func (l *lexer) hex(lval *yySymType) int {
	s := string(l.val)
	h, err := mysql.ParseHex(s)
	if err != nil {
		l.errf("hexadecimal literal: %v", err)
		return int(unicode.ReplacementChar)
	}
	lval.item = h
	return hexLit
}

// https://dev.mysql.com/doc/refman/5.7/en/bit-type.html
func (l *lexer) bit(lval *yySymType) int {
	s := string(l.val)
	b, err := mysql.ParseBit(s, -1)
	if err != nil {
		l.errf("bit literal: %v", err)
		return int(unicode.ReplacementChar)
	}
	lval.item = b
	return bitLit
}

func (l *lexer) handleIdent(lval *yySymType) int {
	s := lval.item.(string)
	// A character string literal may have an optional character set introducer and COLLATE clause:
	// [_charset_name]'string' [COLLATE collation_name]
	// See: https://dev.mysql.com/doc/refman/5.7/en/charset-literal.html
	if !strings.HasPrefix(s, "_") {
		return identifier	
	}
	cs, _, err := charset.GetCharsetInfo(s[1:])
	if err != nil {
		return identifier	
	}
	lval.item = cs
	return underscoreCS
}
