package j

import (
	. "github.com/alecthomas/chroma" // nolint
	"github.com/alecthomas/chroma/lexers/internal"
)

// Julia lexer.
var Julia = internal.Register(MustNewLexer(
	&Config{
		Name:      "Julia",
		Aliases:   []string{"julia", "jl"},
		Filenames: []string{"*.jl"},
		MimeTypes: []string{"text/x-julia", "application/x-julia"},
	},
	Rules{
		"root": {
			{`\n`, Text, nil},
			{`[^\S\n]+`, Text, nil},
			{`#=`, CommentMultiline, Push("blockcomment")},
			{`#.*$`, Comment, nil},
			{`[\[\]{}(),;]`, Punctuation, nil},
			{`in\b`, KeywordPseudo, nil},
			{`isa\b`, KeywordPseudo, nil},
			{`(true|false)\b`, KeywordConstant, nil},
			{`(local|global|const)\b`, KeywordDeclaration, nil},
			{Words(``, `\b`, `function`, `abstract type`, `primitive type`, `baremodule`, `begin`, `bitstype`, `break`, `catch`, `ccall`, `continue`, `do`, `else`, `elseif`, `end`, `export`, `finally`, `for`, `if`, `import`, `let`, `macro`, `module`, `mutable`, `quote`, `return`, `struct`, `try`, `using`, `while`), Keyword, nil},
			{Words(``, `\b`, `ASCIIString`, `AbstractArray`, `AbstractChannel`, `AbstractDict`, `AbstractFloat`, `AbstractMatrix`, `AbstractRNG`, `AbstractSparseArray`, `AbstractSparseMatrix`, `AbstractSparseVector`, `AbstractString`, `AbstractVecOrMat`, `AbstractVector`, `Any`, `ArgumentError`, `Array`, `AssertionError`, `Base64DecodePipe`, `Base64EncodePipe`, `Bidiagonal`, `BigFloat`, `BigInt`, `BitArray`, `BitMatrix`, `BitVector`, `Bool`, `BoundsError`, `Box`, `BufferStream`, `CapturedException`, `CartesianIndex`, `CartesianRange`, `Cchar`, `Cdouble`, `Cfloat`, `Channel`, `Char`, `Cint`, `Cintmax_t`, `Clong`, `Clonglong`, `ClusterManager`, `Cmd`, `Coff_t`, `Colon`, `Complex`, `Complex128`, `Complex32`, `Complex64`, `CompositeException`, `Condition`, `Cptrdiff_t`, `Cshort`, `Csize_t`, `Cssize_t`, `Cstring`, `Cuchar`, `Cuint`, `Cuintmax_t`, `Culong`, `Culonglong`, `Cushort`, `Cwchar_t`, `Cwstring`, `DataType`, `Date`, `DateTime`, `DenseArray`, `DenseMatrix`, `DenseVecOrMat`, `DenseVector`, `Diagonal`, `Dict`, `DimensionMismatch`, `Dims`, `DirectIndexString`, `Display`, `DivideError`, `DomainError`, `EOFError`, `EachLine`, `Enum`, `Enumerate`, `ErrorException`, `Exception`, `Expr`, `Factorization`, `FileMonitor`, `FileOffset`, `Filter`, `Float16`, `Float32`, `Float64`, `FloatRange`, `Function`, `GenSym`, `GlobalRef`, `GotoNode`, `HTML`, `Hermitian`, `IO`, `IOBuffer`, `IOStream`, `IPv4`, `IPv6`, `InexactError`, `InitError`, `Int`, `Int128`, `Int16`, `Int32`, `Int64`, `Int8`, `IntSet`, `Integer`, `InterruptException`, `IntrinsicFunction`, `InvalidStateException`, `Irrational`, `KeyError`, `LabelNode`, `LambdaStaticData`, `LinSpace`, `LineNumberNode`, `LoadError`, `LocalProcess`, `LowerTriangular`, `MIME`, `Matrix`, `MersenneTwister`, `Method`, `MethodError`, `MethodTable`, `Module`, `NTuple`, `NewvarNode`, `NullException`, `Nullable`, `Number`, `ObjectIdDict`, `OrdinalRange`, `OutOfMemoryError`, `OverflowError`, `Pair`, `ParseError`, `PartialQuickSort`, `Pipe`, `PollingFileWatcher`, `ProcessExitedException`, `ProcessGroup`, `Ptr`, `QuoteNode`, `RandomDevice`, `Range`, `Rational`, `RawFD`, `ReadOnlyMemoryError`, `Real`, `ReentrantLock`, `Ref`, `Regex`, `RegexMatch`, `RemoteException`, `RemoteRef`, `RepString`, `RevString`, `RopeString`, `RoundingMode`, `SegmentationFault`, `SerializationState`, `Set`, `SharedArray`, `SharedMatrix`, `SharedVector`, `Signed`, `SimpleVector`, `SparseMatrixCSC`, `StackOverflowError`, `StatStruct`, `StepRange`, `StridedArray`, `StridedMatrix`, `StridedVecOrMat`, `StridedVector`, `SubArray`, `SubString`, `SymTridiagonal`, `Symbol`, `SymbolNode`, `Symmetric`, `SystemError`, `TCPSocket`, `Task`, `Text`, `TextDisplay`, `Timer`, `TopNode`, `Tridiagonal`, `Tuple`, `Type`, `TypeConstructor`, `TypeError`, `TypeName`, `TypeVar`, `UDPSocket`, `UInt`, `UInt128`, `UInt16`, `UInt32`, `UInt64`, `UInt8`, `UTF16String`, `UTF32String`, `UTF8String`, `UndefRefError`, `UndefVarError`, `UnicodeError`, `UniformScaling`, `Union`, `UnitRange`, `Unsigned`, `UpperTriangular`, `Val`, `Vararg`, `VecOrMat`, `Vector`, `VersionNumber`, `Void`, `WString`, `WeakKeyDict`, `WeakRef`, `WorkerConfig`, `Zip`), KeywordType, nil},
			{Words(``, `\b`, `ARGS`, `CPU_CORES`, `C_NULL`, `DevNull`, `ENDIAN_BOM`, `ENV`, `I`, `Inf`, `Inf16`, `Inf32`, `Inf64`, `InsertionSort`, `JULIA_HOME`, `LOAD_PATH`, `MergeSort`, `NaN`, `NaN16`, `NaN32`, `NaN64`, `OS_NAME`, `QuickSort`, `RoundDown`, `RoundFromZero`, `RoundNearest`, `RoundNearestTiesAway`, `RoundNearestTiesUp`, `RoundToZero`, `RoundUp`, `STDERR`, `STDIN`, `STDOUT`, `VERSION`, `WORD_SIZE`, `catalan`, `e`, `eu`, `eulergamma`, `golden`, `im`, `nothing`, `pi`, `γ`, `π`, `φ`), NameBuiltin, nil},
			{Words(``, ``, `=`, `:=`, `+=`, `-=`, `*=`, `/=`, `//=`, `.//=`, `.*=`, `./=`, `\=`, `.\=`, `^=`, `.^=`, `÷=`, `.÷=`, `%=`, `.%=`, `|=`, `&=`, `$=`, `=>`, `<<=`, `>>=`, `>>>=`, `~`, `.+=`, `.-=`, `?`, `--`, `-->`, `||`, `&&`, `>`, `<`, `>=`, `≥`, `<=`, `≤`, `==`, `===`, `≡`, `!=`, `≠`, `!==`, `≢`, `.>`, `.<`, `.>=`, `.≥`, `.<=`, `.≤`, `.==`, `.!=`, `.≠`, `.=`, `.!`, `<:`, `>:`, `∈`, `∉`, `∋`, `∌`, `⊆`, `⊈`, `⊂`, `⊄`, `⊊`, `|>`, `<|`, `:`, `+`, `-`, `.+`, `.-`, `|`, `∪`, `$`, `<<`, `>>`, `>>>`, `.<<`, `.>>`, `.>>>`, `*`, `/`, `./`, `÷`, `.÷`, `%`, `⋅`, `.%`, `.*`, `\`, `.\`, `&`, `∩`, `//`, `.//`, `^`, `.^`, `::`, `.`, `+`, `-`, `!`, `√`, `∛`, `∜`), Operator, nil},
			{`'(\\.|\\[0-7]{1,3}|\\x[a-fA-F0-9]{1,3}|\\u[a-fA-F0-9]{1,4}|\\U[a-fA-F0-9]{1,6}|[^\\\'\n])'`, LiteralStringChar, nil},
			{`(?<=[.\w)\]])\'+`, Operator, nil},
			{`"""`, LiteralString, Push("tqstring")},
			{`"`, LiteralString, Push("string")},
			{`r"""`, LiteralStringRegex, Push("tqregex")},
			{`r"`, LiteralStringRegex, Push("regex")},
			{"`", LiteralStringBacktick, Push("command")},
			{`((?:[a-zA-Z_¡-￿]|[𐀀-􏿿])(?:[a-zA-Z_0-9¡-￿]|[𐀀-􏿿])*!*)(')?`, ByGroups(Name, Operator), nil},
			{`(@(?:[a-zA-Z_¡-￿]|[𐀀-􏿿])(?:[a-zA-Z_0-9¡-￿]|[𐀀-􏿿])*!*)(')?`, ByGroups(NameDecorator, Operator), nil},
			{`(\d+(_\d+)+\.\d*|\d*\.\d+(_\d+)+)([eEf][+-]?[0-9]+)?`, LiteralNumberFloat, nil},
			{`(\d+\.\d*|\d*\.\d+)([eEf][+-]?[0-9]+)?`, LiteralNumberFloat, nil},
			{`\d+(_\d+)+[eEf][+-]?[0-9]+`, LiteralNumberFloat, nil},
			{`\d+[eEf][+-]?[0-9]+`, LiteralNumberFloat, nil},
			{`0b[01]+(_[01]+)+`, LiteralNumberBin, nil},
			{`0b[01]+`, LiteralNumberBin, nil},
			{`0o[0-7]+(_[0-7]+)+`, LiteralNumberOct, nil},
			{`0o[0-7]+`, LiteralNumberOct, nil},
			{`0x[a-fA-F0-9]+(_[a-fA-F0-9]+)+`, LiteralNumberHex, nil},
			{`0x[a-fA-F0-9]+`, LiteralNumberHex, nil},
			{`\d+(_\d+)+`, LiteralNumberInteger, nil},
			{`\d+`, LiteralNumberInteger, nil},
		},
		"blockcomment": {
			{`[^=#]`, CommentMultiline, nil},
			{`#=`, CommentMultiline, Push()},
			{`=#`, CommentMultiline, Pop(1)},
			{`[=#]`, CommentMultiline, nil},
		},
		"string": {
			{`"`, LiteralString, Pop(1)},
			{`\\([\\"\'$nrbtfav]|(x|u|U)[a-fA-F0-9]+|\d+)`, LiteralStringEscape, nil},
			{`\$(?:[a-zA-Z_¡-￿]|[𐀀-􏿿])(?:[a-zA-Z_0-9¡-￿]|[𐀀-􏿿])*!*`, LiteralStringInterpol, nil},
			{`(\$)(\()`, ByGroups(LiteralStringInterpol, Punctuation), Push("in-intp")},
			{`%[-#0 +]*([0-9]+|[*])?(\.([0-9]+|[*]))?[hlL]?[E-GXc-giorsux%]`, LiteralStringInterpol, nil},
			{`.|\s`, LiteralString, nil},
		},
		"tqstring": {
			{`"""`, LiteralString, Pop(1)},
			{`\\([\\"\'$nrbtfav]|(x|u|U)[a-fA-F0-9]+|\d+)`, LiteralStringEscape, nil},
			{`\$(?:[a-zA-Z_¡-￿]|[𐀀-􏿿])(?:[a-zA-Z_0-9¡-￿]|[𐀀-􏿿])*!*`, LiteralStringInterpol, nil},
			{`(\$)(\()`, ByGroups(LiteralStringInterpol, Punctuation), Push("in-intp")},
			{`.|\s`, LiteralString, nil},
		},
		"regex": {
			{`"`, LiteralStringRegex, Pop(1)},
			{`\\"`, LiteralStringRegex, nil},
			{`.|\s`, LiteralStringRegex, nil},
		},
		"tqregex": {
			{`"""`, LiteralStringRegex, Pop(1)},
			{`.|\s`, LiteralStringRegex, nil},
		},
		"command": {
			{"`", LiteralStringBacktick, Pop(1)},
			{`\$(?:[a-zA-Z_¡-￿]|[𐀀-􏿿])(?:[a-zA-Z_0-9¡-￿]|[𐀀-􏿿])*!*`, LiteralStringInterpol, nil},
			{`(\$)(\()`, ByGroups(LiteralStringInterpol, Punctuation), Push("in-intp")},
			{`.|\s`, LiteralStringBacktick, nil},
		},
		"in-intp": {
			{`\(`, Punctuation, Push()},
			{`\)`, Punctuation, Pop(1)},
			Include("root"),
		},
	},
))
