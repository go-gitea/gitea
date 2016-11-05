package gls

// so, basically, we're going to encode integer tags in base-16 on the stack

const (
	bitWidth = 4
)

func addStackTag(tag uint, context_call func()) {
	if context_call == nil {
		return
	}
	markS(tag, context_call)
}

func markS(tag uint, cb func()) { _m(tag, cb) }
func mark0(tag uint, cb func()) { _m(tag, cb) }
func mark1(tag uint, cb func()) { _m(tag, cb) }
func mark2(tag uint, cb func()) { _m(tag, cb) }
func mark3(tag uint, cb func()) { _m(tag, cb) }
func mark4(tag uint, cb func()) { _m(tag, cb) }
func mark5(tag uint, cb func()) { _m(tag, cb) }
func mark6(tag uint, cb func()) { _m(tag, cb) }
func mark7(tag uint, cb func()) { _m(tag, cb) }
func mark8(tag uint, cb func()) { _m(tag, cb) }
func mark9(tag uint, cb func()) { _m(tag, cb) }
func markA(tag uint, cb func()) { _m(tag, cb) }
func markB(tag uint, cb func()) { _m(tag, cb) }
func markC(tag uint, cb func()) { _m(tag, cb) }
func markD(tag uint, cb func()) { _m(tag, cb) }
func markE(tag uint, cb func()) { _m(tag, cb) }
func markF(tag uint, cb func()) { _m(tag, cb) }

var pc_lookup = make(map[uintptr]int8, 17)
var mark_lookup [16]func(uint, func())

func _m(tag_remainder uint, cb func()) {
	if tag_remainder == 0 {
		cb()
	} else {
		mark_lookup[tag_remainder&0xf](tag_remainder>>bitWidth, cb)
	}
}
