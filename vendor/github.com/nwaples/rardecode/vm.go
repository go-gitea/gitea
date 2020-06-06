package rardecode

import (
	"encoding/binary"
	"errors"
)

const (
	// vm flag bits
	flagC = 1          // Carry
	flagZ = 2          // Zero
	flagS = 0x80000000 // Sign

	maxCommands = 25000000 // maximum number of commands that can be run in a program

	vmRegs = 8       // number if registers
	vmSize = 0x40000 // memory size
	vmMask = vmSize - 1
)

var (
	errInvalidVMInstruction = errors.New("rardecode: invalid vm instruction")
)

type vm struct {
	ip    uint32         // instruction pointer
	ipMod bool           // ip was modified
	fl    uint32         // flag bits
	r     [vmRegs]uint32 // registers
	m     []byte         // memory
}

func (v *vm) setIP(ip uint32) {
	v.ip = ip
	v.ipMod = true
}

// execute runs a list of commands on the vm.
func (v *vm) execute(cmd []command) {
	v.ip = 0 // reset instruction pointer
	for n := 0; n < maxCommands; n++ {
		ip := v.ip
		if ip >= uint32(len(cmd)) {
			return
		}
		ins := cmd[ip]
		ins.f(v, ins.bm, ins.op) // run cpu instruction
		if v.ipMod {
			// command modified ip, don't increment
			v.ipMod = false
		} else {
			v.ip++ // increment ip for next command
		}
	}
}

// newVM creates a new RAR virtual machine using the byte slice as memory.
func newVM(mem []byte) *vm {
	v := new(vm)

	if cap(mem) < vmSize+4 {
		v.m = make([]byte, vmSize+4)
		copy(v.m, mem)
	} else {
		v.m = mem[:vmSize+4]
		for i := len(mem); i < len(v.m); i++ {
			v.m[i] = 0
		}
	}
	v.r[7] = vmSize
	return v
}

type operand interface {
	get(v *vm, byteMode bool) uint32
	set(v *vm, byteMode bool, n uint32)
}

// Immediate Operand
type opI uint32

func (op opI) get(v *vm, bm bool) uint32    { return uint32(op) }
func (op opI) set(v *vm, bm bool, n uint32) {}

// Direct Operand
type opD uint32

func (op opD) get(v *vm, byteMode bool) uint32 {
	if byteMode {
		return uint32(v.m[op])
	}
	return binary.LittleEndian.Uint32(v.m[op:])
}

func (op opD) set(v *vm, byteMode bool, n uint32) {
	if byteMode {
		v.m[op] = byte(n)
	} else {
		binary.LittleEndian.PutUint32(v.m[op:], n)
	}
}

// Register  Operand
type opR uint32

func (op opR) get(v *vm, byteMode bool) uint32 {
	if byteMode {
		return v.r[op] & 0xFF
	}
	return v.r[op]
}

func (op opR) set(v *vm, byteMode bool, n uint32) {
	if byteMode {
		v.r[op] = (v.r[op] & 0xFFFFFF00) | (n & 0xFF)
	} else {
		v.r[op] = n
	}
}

// Register Indirect Operand
type opRI uint32

func (op opRI) get(v *vm, byteMode bool) uint32 {
	i := v.r[op] & vmMask
	if byteMode {
		return uint32(v.m[i])
	}
	return binary.LittleEndian.Uint32(v.m[i:])
}
func (op opRI) set(v *vm, byteMode bool, n uint32) {
	i := v.r[op] & vmMask
	if byteMode {
		v.m[i] = byte(n)
	} else {
		binary.LittleEndian.PutUint32(v.m[i:], n)
	}
}

// Base Plus Index Indirect Operand
type opBI struct {
	r uint32
	i uint32
}

func (op opBI) get(v *vm, byteMode bool) uint32 {
	i := (v.r[op.r] + op.i) & vmMask
	if byteMode {
		return uint32(v.m[i])
	}
	return binary.LittleEndian.Uint32(v.m[i:])
}
func (op opBI) set(v *vm, byteMode bool, n uint32) {
	i := (v.r[op.r] + op.i) & vmMask
	if byteMode {
		v.m[i] = byte(n)
	} else {
		binary.LittleEndian.PutUint32(v.m[i:], n)
	}
}

type commandFunc func(v *vm, byteMode bool, op []operand)

type command struct {
	f  commandFunc
	bm bool // is byte mode
	op []operand
}

var (
	ops = []struct {
		f        commandFunc
		byteMode bool // supports byte mode
		nops     int  // number of operands
		jop      bool // is a jump op
	}{
		{mov, true, 2, false},
		{cmp, true, 2, false},
		{add, true, 2, false},
		{sub, true, 2, false},
		{jz, false, 1, true},
		{jnz, false, 1, true},
		{inc, true, 1, false},
		{dec, true, 1, false},
		{jmp, false, 1, true},
		{xor, true, 2, false},
		{and, true, 2, false},
		{or, true, 2, false},
		{test, true, 2, false},
		{js, false, 1, true},
		{jns, false, 1, true},
		{jb, false, 1, true},
		{jbe, false, 1, true},
		{ja, false, 1, true},
		{jae, false, 1, true},
		{push, false, 1, false},
		{pop, false, 1, false},
		{call, false, 1, true},
		{ret, false, 0, false},
		{not, true, 1, false},
		{shl, true, 2, false},
		{shr, true, 2, false},
		{sar, true, 2, false},
		{neg, true, 1, false},
		{pusha, false, 0, false},
		{popa, false, 0, false},
		{pushf, false, 0, false},
		{popf, false, 0, false},
		{movzx, false, 2, false},
		{movsx, false, 2, false},
		{xchg, true, 2, false},
		{mul, true, 2, false},
		{div, true, 2, false},
		{adc, true, 2, false},
		{sbb, true, 2, false},
		{print, false, 0, false},
	}
)

func mov(v *vm, bm bool, op []operand) {
	op[0].set(v, bm, op[1].get(v, bm))
}

func cmp(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	r := v1 - op[1].get(v, bm)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = 0
		if r > v1 {
			v.fl = flagC
		}
		v.fl |= r & flagS
	}
}

func add(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	r := v1 + op[1].get(v, bm)
	v.fl = 0
	signBit := uint32(flagS)
	if bm {
		r &= 0xFF
		signBit = 0x80
	}
	if r < v1 {
		v.fl |= flagC
	}
	if r == 0 {
		v.fl |= flagZ
	} else if r&signBit > 0 {
		v.fl |= flagS
	}
	op[0].set(v, bm, r)
}

func sub(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	r := v1 - op[1].get(v, bm)
	v.fl = 0

	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = 0
		if r > v1 {
			v.fl = flagC
		}
		v.fl |= r & flagS
	}
	op[0].set(v, bm, r)
}

func jz(v *vm, bm bool, op []operand) {
	if v.fl&flagZ > 0 {
		v.setIP(op[0].get(v, false))
	}
}

func jnz(v *vm, bm bool, op []operand) {
	if v.fl&flagZ == 0 {
		v.setIP(op[0].get(v, false))
	}
}

func inc(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) + 1
	if bm {
		r &= 0xFF
	}
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func dec(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) - 1
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func jmp(v *vm, bm bool, op []operand) {
	v.setIP(op[0].get(v, false))
}

func xor(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) ^ op[1].get(v, bm)
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func and(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) & op[1].get(v, bm)
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func or(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) | op[1].get(v, bm)
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func test(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) & op[1].get(v, bm)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
}

func js(v *vm, bm bool, op []operand) {
	if v.fl&flagS > 0 {
		v.setIP(op[0].get(v, false))
	}
}

func jns(v *vm, bm bool, op []operand) {
	if v.fl&flagS == 0 {
		v.setIP(op[0].get(v, false))
	}
}

func jb(v *vm, bm bool, op []operand) {
	if v.fl&flagC > 0 {
		v.setIP(op[0].get(v, false))
	}
}

func jbe(v *vm, bm bool, op []operand) {
	if v.fl&(flagC|flagZ) > 0 {
		v.setIP(op[0].get(v, false))
	}
}

func ja(v *vm, bm bool, op []operand) {
	if v.fl&(flagC|flagZ) == 0 {
		v.setIP(op[0].get(v, false))
	}
}

func jae(v *vm, bm bool, op []operand) {
	if v.fl&flagC == 0 {
		v.setIP(op[0].get(v, false))
	}
}

func push(v *vm, bm bool, op []operand) {
	v.r[7] -= 4
	opRI(7).set(v, false, op[0].get(v, false))

}

func pop(v *vm, bm bool, op []operand) {
	op[0].set(v, false, opRI(7).get(v, false))
	v.r[7] += 4
}

func call(v *vm, bm bool, op []operand) {
	v.r[7] -= 4
	opRI(7).set(v, false, v.ip+1)
	v.setIP(op[0].get(v, false))
}

func ret(v *vm, bm bool, op []operand) {
	r7 := v.r[7]
	if r7 >= vmSize {
		v.setIP(0xFFFFFFFF) // trigger end of program
	} else {
		v.setIP(binary.LittleEndian.Uint32(v.m[r7:]))
		v.r[7] += 4
	}
}

func not(v *vm, bm bool, op []operand) {
	op[0].set(v, bm, ^op[0].get(v, bm))
}

func shl(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	v2 := op[1].get(v, bm)
	r := v1 << v2
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
	if (v1<<(v2-1))&0x80000000 > 0 {
		v.fl |= flagC
	}
}

func shr(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	v2 := op[1].get(v, bm)
	r := v1 >> v2
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
	if (v1>>(v2-1))&0x1 > 0 {
		v.fl |= flagC
	}
}

func sar(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	v2 := op[1].get(v, bm)
	r := uint32(int32(v1) >> v2)
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
	if (v1>>(v2-1))&0x1 > 0 {
		v.fl |= flagC
	}
}

func neg(v *vm, bm bool, op []operand) {
	r := 0 - op[0].get(v, bm)
	op[0].set(v, bm, r)
	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r&flagS | flagC
	}
}

func pusha(v *vm, bm bool, op []operand) {
	sp := opD(v.r[7])
	for _, r := range v.r {
		sp = (sp - 4) & vmMask
		sp.set(v, false, r)
	}
	v.r[7] = uint32(sp)
}

func popa(v *vm, bm bool, op []operand) {
	sp := opD(v.r[7])
	for i := 7; i >= 0; i-- {
		v.r[i] = sp.get(v, false)
		sp = (sp + 4) & vmMask
	}
}

func pushf(v *vm, bm bool, op []operand) {
	v.r[7] -= 4
	opRI(7).set(v, false, v.fl)
}

func popf(v *vm, bm bool, op []operand) {
	v.fl = opRI(7).get(v, false)
	v.r[7] += 4
}

func movzx(v *vm, bm bool, op []operand) {
	op[0].set(v, false, op[1].get(v, true))
}

func movsx(v *vm, bm bool, op []operand) {
	op[0].set(v, false, uint32(int8(op[1].get(v, true))))
}

func xchg(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	op[0].set(v, bm, op[1].get(v, bm))
	op[1].set(v, bm, v1)
}

func mul(v *vm, bm bool, op []operand) {
	r := op[0].get(v, bm) * op[1].get(v, bm)
	op[0].set(v, bm, r)
}

func div(v *vm, bm bool, op []operand) {
	div := op[1].get(v, bm)
	if div != 0 {
		r := op[0].get(v, bm) / div
		op[0].set(v, bm, r)
	}
}

func adc(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	fc := v.fl & flagC
	r := v1 + op[1].get(v, bm) + fc
	if bm {
		r &= 0xFF
	}
	op[0].set(v, bm, r)

	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
	if r < v1 || (r == v1 && fc > 0) {
		v.fl |= flagC
	}
}

func sbb(v *vm, bm bool, op []operand) {
	v1 := op[0].get(v, bm)
	fc := v.fl & flagC
	r := v1 - op[1].get(v, bm) - fc
	if bm {
		r &= 0xFF
	}
	op[0].set(v, bm, r)

	if r == 0 {
		v.fl = flagZ
	} else {
		v.fl = r & flagS
	}
	if r > v1 || (r == v1 && fc > 0) {
		v.fl |= flagC
	}
}

func print(v *vm, bm bool, op []operand) {
	// TODO: ignore print for the moment
}

func decodeArg(br *rarBitReader, byteMode bool) (operand, error) {
	n, err := br.readBits(1)
	if err != nil {
		return nil, err
	}
	if n > 0 { // Register
		n, err = br.readBits(3)
		return opR(n), err
	}
	n, err = br.readBits(1)
	if err != nil {
		return nil, err
	}
	if n == 0 { // Immediate
		if byteMode {
			n, err = br.readBits(8)
		} else {
			m, err := br.readUint32()
			return opI(m), err
		}
		return opI(n), err
	}
	n, err = br.readBits(1)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		// Register Indirect
		n, err = br.readBits(3)
		return opRI(n), err
	}
	n, err = br.readBits(1)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		// Base + Index Indirect
		n, err = br.readBits(3)
		if err != nil {
			return nil, err
		}
		i, err := br.readUint32()
		return opBI{r: uint32(n), i: i}, err
	}
	// Direct addressing
	m, err := br.readUint32()
	return opD(m & vmMask), err
}

func fixJumpOp(op operand, off int) operand {
	n, ok := op.(opI)
	if !ok {
		return op
	}
	if n >= 256 {
		return n - 256
	}
	if n >= 136 {
		n -= 264
	} else if n >= 16 {
		n -= 8
	} else if n >= 8 {
		n -= 16
	}
	return n + opI(off)
}

func readCommands(br *rarBitReader) ([]command, error) {
	var cmds []command

	for {
		code, err := br.readBits(4)
		if err != nil {
			return cmds, err
		}
		if code&0x08 > 0 {
			n, err := br.readBits(2)
			if err != nil {
				return cmds, err
			}
			code = (code<<2 | n) - 24
		}

		if code >= len(ops) {
			return cmds, errInvalidVMInstruction
		}
		ins := ops[code]

		var com command

		if ins.byteMode {
			n, err := br.readBits(1)
			if err != nil {
				return cmds, err
			}
			com.bm = n > 0
		}
		com.f = ins.f

		if ins.nops > 0 {
			com.op = make([]operand, ins.nops)
			com.op[0], err = decodeArg(br, com.bm)
			if err != nil {
				return cmds, err
			}
			if ins.nops == 2 {
				com.op[1], err = decodeArg(br, com.bm)
				if err != nil {
					return cmds, err
				}
			} else if ins.jop {
				com.op[0] = fixJumpOp(com.op[0], len(cmds))
			}
		}
		cmds = append(cmds, com)
	}
}
