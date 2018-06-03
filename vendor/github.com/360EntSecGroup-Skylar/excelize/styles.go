package excelize

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Excel styles can reference number formats that are built-in, all of which
// have an id less than 164. This is a possibly incomplete list comprised of as
// many of them as I could find.
var builtInNumFmt = map[int]string{
	0:  "general",
	1:  "0",
	2:  "0.00",
	3:  "#,##0",
	4:  "#,##0.00",
	9:  "0%",
	10: "0.00%",
	11: "0.00e+00",
	12: "# ?/?",
	13: "# ??/??",
	14: "mm-dd-yy",
	15: "d-mmm-yy",
	16: "d-mmm",
	17: "mmm-yy",
	18: "h:mm am/pm",
	19: "h:mm:ss am/pm",
	20: "h:mm",
	21: "h:mm:ss",
	22: "m/d/yy h:mm",
	37: "#,##0 ;(#,##0)",
	38: "#,##0 ;[red](#,##0)",
	39: "#,##0.00;(#,##0.00)",
	40: "#,##0.00;[red](#,##0.00)",
	41: `_(* #,##0_);_(* \(#,##0\);_(* "-"_);_(@_)`,
	42: `_("$"* #,##0_);_("$* \(#,##0\);_("$"* "-"_);_(@_)`,
	43: `_(* #,##0.00_);_(* \(#,##0.00\);_(* "-"??_);_(@_)`,
	44: `_("$"* #,##0.00_);_("$"* \(#,##0.00\);_("$"* "-"??_);_(@_)`,
	45: "mm:ss",
	46: "[h]:mm:ss",
	47: "mmss.0",
	48: "##0.0e+0",
	49: "@",
}

// langNumFmt defined number format code (with unicode values provided for
// language glyphs where they occur) in different language.
var langNumFmt = map[string]map[int]string{
	"zh-tw": {
		27: "[$-404]e/m/d",
		28: `[$-404]e"年"m"月"d"日"`,
		29: `[$-404]e"年"m"月"d"日"`,
		30: "m/d/yy",
		31: `yyyy"年"m"月"d"日"`,
		32: `hh"時"mm"分"`,
		33: `hh"時"mm"分"ss"秒"`,
		34: `上午/下午 hh"時"mm"分"`,
		35: `上午/下午 hh"時"mm"分"ss"秒"`,
		36: "[$-404]e/m/d",
		50: "[$-404]e/m/d",
		51: `[$-404]e"年"m"月"d"日"`,
		52: `上午/下午 hh"時"mm"分"`,
		53: `上午/下午 hh"時"mm"分"ss"秒"`,
		54: `[$-404]e"年"m"月"d"日"`,
		55: `上午/下午 hh"時"mm"分"`,
		56: `上午/下午 hh"時"mm"分"ss"秒"`,
		57: "[$-404]e/m/d",
		58: `[$-404]e"年"m"月"d"日"`,
	},
	"zh-cn": {
		27: `yyyy"年"m"月"`,
		28: `m"月"d"日"`,
		29: `m"月"d"日"`,
		30: "m-d-yy",
		31: `yyyy"年"m"月"d"日"`,
		32: `h"时"mm"分"`,
		33: `h"时"mm"分"ss"秒"`,
		34: `上午/下午 h"时"mm"分"`,
		35: `上午/下午 h"时"mm"分"ss"秒"`,
		36: `yyyy"年"m"月"`,
		50: `yyyy"年"m"月"`,
		51: `m"月"d"日"`,
		52: `yyyy"年"m"月"`,
		53: `m"月"d"日"`,
		54: `m"月"d"日"`,
		55: `上午/下午 h"时"mm"分"`,
		56: `上午/下午 h"时"mm"分"ss"秒"`,
		57: `yyyy"年"m"月"`,
		58: `m"月"d"日"`,
	},
	"zh-tw_unicode": {
		27: "[$-404]e/m/d",
		28: `[$-404]e"5E74"m"6708"d"65E5"`,
		29: `[$-404]e"5E74"m"6708"d"65E5"`,
		30: "m/d/yy",
		31: `yyyy"5E74"m"6708"d"65E5"`,
		32: `hh"6642"mm"5206"`,
		33: `hh"6642"mm"5206"ss"79D2"`,
		34: `4E0A5348/4E0B5348hh"6642"mm"5206"`,
		35: `4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2"`,
		36: "[$-404]e/m/d",
		50: "[$-404]e/m/d",
		51: `[$-404]e"5E74"m"6708"d"65E5"`,
		52: `4E0A5348/4E0B5348hh"6642"mm"5206"`,
		53: `4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2"`,
		54: `[$-404]e"5E74"m"6708"d"65E5"`,
		55: `4E0A5348/4E0B5348hh"6642"mm"5206"`,
		56: `4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2"`,
		57: "[$-404]e/m/d",
		58: `[$-404]e"5E74"m"6708"d"65E5"`,
	},
	"zh-cn_unicode": {
		27: `yyyy"5E74"m"6708"`,
		28: `m"6708"d"65E5"`,
		29: `m"6708"d"65E5"`,
		30: "m-d-yy",
		31: `yyyy"5E74"m"6708"d"65E5"`,
		32: `h"65F6"mm"5206"`,
		33: `h"65F6"mm"5206"ss"79D2"`,
		34: `4E0A5348/4E0B5348h"65F6"mm"5206"`,
		35: `4E0A5348/4E0B5348h"65F6"mm"5206"ss"79D2"`,
		36: `yyyy"5E74"m"6708"`,
		50: `yyyy"5E74"m"6708"`,
		51: `m"6708"d"65E5"`,
		52: `yyyy"5E74"m"6708"`,
		53: `m"6708"d"65E5"`,
		54: `m"6708"d"65E5"`,
		55: `4E0A5348/4E0B5348h"65F6"mm"5206"`,
		56: `4E0A5348/4E0B5348h"65F6"mm"5206"ss"79D2"`,
		57: `yyyy"5E74"m"6708"`,
		58: `m"6708"d"65E5"`,
	},
	"ja-jp": {
		27: "[$-411]ge.m.d",
		28: `[$-411]ggge"年"m"月"d"日"`,
		29: `[$-411]ggge"年"m"月"d"日"`,
		30: "m/d/yy",
		31: `yyyy"年"m"月"d"日"`,
		32: `h"時"mm"分"`,
		33: `h"時"mm"分"ss"秒"`,
		34: `yyyy"年"m"月"`,
		35: `m"月"d"日"`,
		36: "[$-411]ge.m.d",
		50: "[$-411]ge.m.d",
		51: `[$-411]ggge"年"m"月"d"日"`,
		52: `yyyy"年"m"月"`,
		53: `m"月"d"日"`,
		54: `[$-411]ggge"年"m"月"d"日"`,
		55: `yyyy"年"m"月"`,
		56: `m"月"d"日"`,
		57: "[$-411]ge.m.d",
		58: `[$-411]ggge"年"m"月"d"日"`,
	},
	"ko-kr": {
		27: `yyyy"年" mm"月" dd"日"`,
		28: "mm-dd",
		29: "mm-dd",
		30: "mm-dd-yy",
		31: `yyyy"년" mm"월" dd"일"`,
		32: `h"시" mm"분"`,
		33: `h"시" mm"분" ss"초"`,
		34: `yyyy-mm-dd`,
		35: `yyyy-mm-dd`,
		36: `yyyy"年" mm"月" dd"日"`,
		50: `yyyy"年" mm"月" dd"日"`,
		51: "mm-dd",
		52: "yyyy-mm-dd",
		53: "yyyy-mm-dd",
		54: "mm-dd",
		55: "yyyy-mm-dd",
		56: "yyyy-mm-dd",
		57: `yyyy"年" mm"月" dd"日"`,
		58: "mm-dd",
	},
	"ja-jp_unicode": {
		27: "[$-411]ge.m.d",
		28: `[$-411]ggge"5E74"m"6708"d"65E5"`,
		29: `[$-411]ggge"5E74"m"6708"d"65E5"`,
		30: "m/d/yy",
		31: `yyyy"5E74"m"6708"d"65E5"`,
		32: `h"6642"mm"5206"`,
		33: `h"6642"mm"5206"ss"79D2"`,
		34: `yyyy"5E74"m"6708"`,
		35: `m"6708"d"65E5"`,
		36: "[$-411]ge.m.d",
		50: "[$-411]ge.m.d",
		51: `[$-411]ggge"5E74"m"6708"d"65E5"`,
		52: `yyyy"5E74"m"6708"`,
		53: `m"6708"d"65E5"`,
		54: `[$-411]ggge"5E74"m"6708"d"65E5"`,
		55: `yyyy"5E74"m"6708"`,
		56: `m"6708"d"65E5"`,
		57: "[$-411]ge.m.d",
		58: `[$-411]ggge"5E74"m"6708"d"65E5"`,
	},
	"ko-kr_unicode": {
		27: `yyyy"5E74" mm"6708" dd"65E5"`,
		28: "mm-dd",
		29: "mm-dd",
		30: "mm-dd-yy",
		31: `yyyy"B144" mm"C6D4" dd"C77C"`,
		32: `h"C2DC" mm"BD84"`,
		33: `h"C2DC" mm"BD84" ss"CD08"`,
		34: "yyyy-mm-dd",
		35: "yyyy-mm-dd",
		36: `yyyy"5E74" mm"6708" dd"65E5"`,
		50: `yyyy"5E74" mm"6708" dd"65E5"`,
		51: "mm-dd",
		52: "yyyy-mm-dd",
		53: "yyyy-mm-dd",
		54: "mm-dd",
		55: "yyyy-mm-dd",
		56: "yyyy-mm-dd",
		57: `yyyy"5E74" mm"6708" dd"65E5"`,
		58: "mm-dd",
	},
	"th-th": {
		59: "t0",
		60: "t0.00",
		61: "t#,##0",
		62: "t#,##0.00",
		67: "t0%",
		68: "t0.00%",
		69: "t# ?/?",
		70: "t# ??/??",
		71: "ว/ด/ปปปป",
		72: "ว-ดดด-ปป",
		73: "ว-ดดด",
		74: "ดดด-ปป",
		75: "ช:นน",
		76: "ช:นน:ทท",
		77: "ว/ด/ปปปป ช:นน",
		78: "นน:ทท",
		79: "[ช]:นน:ทท",
		80: "นน:ทท.0",
		81: "d/m/bb",
	},
	"th-th_unicode": {
		59: "t0",
		60: "t0.00",
		61: "t#,##0",
		62: "t#,##0.00",
		67: "t0%",
		68: "t0.00%",
		69: "t# ?/?",
		70: "t# ??/??",
		71: "0E27/0E14/0E1B0E1B0E1B0E1B",
		72: "0E27-0E140E140E14-0E1B0E1B",
		73: "0E27-0E140E140E14",
		74: "0E140E140E14-0E1B0E1B",
		75: "0E0A:0E190E19",
		76: "0E0A:0E190E19:0E170E17",
		77: "0E27/0E14/0E1B0E1B0E1B0E1B 0E0A:0E190E19",
		78: "0E190E19:0E170E17",
		79: "[0E0A]:0E190E19:0E170E17",
		80: "0E190E19:0E170E17.0",
		81: "d/m/bb",
	},
}

// currencyNumFmt defined the currency number format map.
var currencyNumFmt = map[int]string{
	164: `"CN¥",##0.00`,
	165: "[$$-409]#,##0.00",
	166: "[$$-45C]#,##0.00",
	167: "[$$-1004]#,##0.00",
	168: "[$$-404]#,##0.00",
	169: "[$$-C09]#,##0.00",
	170: "[$$-2809]#,##0.00",
	171: "[$$-1009]#,##0.00",
	172: "[$$-2009]#,##0.00",
	173: "[$$-1409]#,##0.00",
	174: "[$$-4809]#,##0.00",
	175: "[$$-2C09]#,##0.00",
	176: "[$$-2409]#,##0.00",
	177: "[$$-1000]#,##0.00",
	178: `#,##0.00\ [$$-C0C]`,
	179: "[$$-475]#,##0.00",
	180: "[$$-83E]#,##0.00",
	181: `[$$-86B]\ #,##0.00`,
	182: `[$$-340A]\ #,##0.00`,
	183: "[$$-240A]#,##0.00",
	184: `[$$-300A]\ #,##0.00`,
	185: "[$$-440A]#,##0.00",
	186: "[$$-80A]#,##0.00",
	187: "[$$-500A]#,##0.00",
	188: "[$$-540A]#,##0.00",
	189: `[$$-380A]\ #,##0.00`,
	190: "[$£-809]#,##0.00",
	191: "[$£-491]#,##0.00",
	192: "[$£-452]#,##0.00",
	193: "[$¥-804]#,##0.00",
	194: "[$¥-411]#,##0.00",
	195: "[$¥-478]#,##0.00",
	196: "[$¥-451]#,##0.00",
	197: "[$¥-480]#,##0.00",
	198: "#,##0.00\\ [$\u058F-42B]",
	199: "[$\u060B-463]#,##0.00",
	200: "[$\u060B-48C]#,##0.00",
	201: "[$\u09F3-845]\\ #,##0.00",
	202: "#,##0.00[$\u17DB-453]",
	203: "[$\u20A1-140A]#,##0.00",
	204: "[$\u20A6-468]\\ #,##0.00",
	205: "[$\u20A6-470]\\ #,##0.00",
	206: "[$\u20A9-412]#,##0.00",
	207: "[$\u20AA-40D]\\ #,##0.00",
	208: "#,##0.00\\ [$\u20AB-42A]",
	209: "#,##0.00\\ [$\u20AC-42D]",
	210: "#,##0.00\\ [$\u20AC-47E]",
	211: "#,##0.00\\ [$\u20AC-403]",
	212: "#,##0.00\\ [$\u20AC-483]",
	213: "[$\u20AC-813]\\ #,##0.00",
	214: "[$\u20AC-413]\\ #,##0.00",
	215: "[$\u20AC-1809]#,##0.00",
	216: "#,##0.00\\ [$\u20AC-425]",
	217: "[$\u20AC-2]\\ #,##0.00",
	218: "#,##0.00\\ [$\u20AC-1]",
	219: "#,##0.00\\ [$\u20AC-40B]",
	220: "#,##0.00\\ [$\u20AC-80C]",
	221: "#,##0.00\\ [$\u20AC-40C]",
	222: "#,##0.00\\ [$\u20AC-140C]",
	223: "#,##0.00\\ [$\u20AC-180C]",
	224: "[$\u20AC-200C]#,##0.00",
	225: "#,##0.00\\ [$\u20AC-456]",
	226: "#,##0.00\\ [$\u20AC-C07]",
	227: "#,##0.00\\ [$\u20AC-407]",
	228: "#,##0.00\\ [$\u20AC-1007]",
	229: "#,##0.00\\ [$\u20AC-408]",
	230: "#,##0.00\\ [$\u20AC-243B]",
	231: "[$\u20AC-83C]#,##0.00",
	232: "[$\u20AC-410]\\ #,##0.00",
	233: "[$\u20AC-476]#,##0.00",
	234: "#,##0.00\\ [$\u20AC-2C1A]",
	235: "[$\u20AC-426]\\ #,##0.00",
	236: "#,##0.00\\ [$\u20AC-427]",
	237: "#,##0.00\\ [$\u20AC-82E]",
	238: "#,##0.00\\ [$\u20AC-46E]",
	239: "[$\u20AC-43A]#,##0.00",
	240: "#,##0.00\\ [$\u20AC-C3B]",
	241: "#,##0.00\\ [$\u20AC-482]",
	242: "#,##0.00\\ [$\u20AC-816]",
	243: "#,##0.00\\ [$\u20AC-301A]",
	244: "#,##0.00\\ [$\u20AC-203B]",
	245: "#,##0.00\\ [$\u20AC-41B]",
	246: "#,##0.00\\ [$\u20AC-424]",
	247: "#,##0.00\\ [$\u20AC-C0A]",
	248: "#,##0.00\\ [$\u20AC-81D]",
	249: "#,##0.00\\ [$\u20AC-484]",
	250: "#,##0.00\\ [$\u20AC-42E]",
	251: "[$\u20AC-462]\\ #,##0.00",
	252: "#,##0.00\\ [$₭-454]",
	253: "#,##0.00\\ [$₮-450]",
	254: "[$\u20AE-C50]#,##0.00",
	255: "[$\u20B1-3409]#,##0.00",
	256: "[$\u20B1-464]#,##0.00",
	257: "#,##0.00[$\u20B4-422]",
	258: "[$\u20B8-43F]#,##0.00",
	259: "[$\u20B9-460]#,##0.00",
	260: "[$\u20B9-4009]\\ #,##0.00",
	261: "[$\u20B9-447]\\ #,##0.00",
	262: "[$\u20B9-439]\\ #,##0.00",
	263: "[$\u20B9-44B]\\ #,##0.00",
	264: "[$\u20B9-860]#,##0.00",
	265: "[$\u20B9-457]\\ #,##0.00",
	266: "[$\u20B9-458]#,##0.00",
	267: "[$\u20B9-44E]\\ #,##0.00",
	268: "[$\u20B9-861]#,##0.00",
	269: "[$\u20B9-448]\\ #,##0.00",
	270: "[$\u20B9-446]\\ #,##0.00",
	271: "[$\u20B9-44F]\\ #,##0.00",
	272: "[$\u20B9-459]#,##0.00",
	273: "[$\u20B9-449]\\ #,##0.00",
	274: "[$\u20B9-820]#,##0.00",
	275: "#,##0.00\\ [$\u20BA-41F]",
	276: "#,##0.00\\ [$\u20BC-42C]",
	277: "#,##0.00\\ [$\u20BC-82C]",
	278: "#,##0.00\\ [$\u20BD-419]",
	279: "#,##0.00[$\u20BD-485]",
	280: "#,##0.00\\ [$\u20BE-437]",
	281: "[$B/.-180A]\\ #,##0.00",
	282: "[$Br-472]#,##0.00",
	283: "[$Br-477]#,##0.00",
	284: "#,##0.00[$Br-473]",
	285: "[$Bs-46B]\\ #,##0.00",
	286: "[$Bs-400A]\\ #,##0.00",
	287: "[$Bs.-200A]\\ #,##0.00",
	288: "[$BWP-832]\\ #,##0.00",
	289: "[$C$-4C0A]#,##0.00",
	290: "[$CA$-85D]#,##0.00",
	291: "[$CA$-47C]#,##0.00",
	292: "[$CA$-45D]#,##0.00",
	293: "[$CFA-340C]#,##0.00",
	294: "[$CFA-280C]#,##0.00",
	295: "#,##0.00\\ [$CFA-867]",
	296: "#,##0.00\\ [$CFA-488]",
	297: "#,##0.00\\ [$CHF-100C]",
	298: "[$CHF-1407]\\ #,##0.00",
	299: "[$CHF-807]\\ #,##0.00",
	300: "[$CHF-810]\\ #,##0.00",
	301: "[$CHF-417]\\ #,##0.00",
	302: "[$CLP-47A]\\ #,##0.00",
	303: "[$CN¥-850]#,##0.00",
	304: "#,##0.00\\ [$DZD-85F]",
	305: "[$FCFA-2C0C]#,##0.00",
	306: "#,##0.00\\ [$Ft-40E]",
	307: "[$G-3C0C]#,##0.00",
	308: "[$Gs.-3C0A]\\ #,##0.00",
	309: "[$GTQ-486]#,##0.00",
	310: "[$HK$-C04]#,##0.00",
	311: "[$HK$-3C09]#,##0.00",
	312: "#,##0.00\\ [$HRK-41A]",
	313: "[$IDR-3809]#,##0.00",
	314: "[$IQD-492]#,##0.00",
	315: "#,##0.00\\ [$ISK-40F]",
	316: "[$K-455]#,##0.00",
	317: "#,##0.00\\ [$K\u010D-405]",
	318: "#,##0.00\\ [$KM-141A]",
	319: "#,##0.00\\ [$KM-101A]",
	320: "#,##0.00\\ [$KM-181A]",
	321: "[$kr-438]\\ #,##0.00",
	322: "[$kr-43B]\\ #,##0.00",
	323: "#,##0.00\\ [$kr-83B]",
	324: "[$kr-414]\\ #,##0.00",
	325: "[$kr-814]\\ #,##0.00",
	326: "#,##0.00\\ [$kr-41D]",
	327: "[$kr.-406]\\ #,##0.00",
	328: "[$kr.-46F]\\ #,##0.00",
	329: "[$Ksh-441]#,##0.00",
	330: "[$L-818]#,##0.00",
	331: "[$L-819]#,##0.00",
	332: "[$L-480A]\\ #,##0.00",
	333: "#,##0.00\\ [$Lek\u00EB-41C]",
	334: "[$MAD-45F]#,##0.00",
	335: "[$MAD-380C]#,##0.00",
	336: "#,##0.00\\ [$MAD-105F]",
	337: "[$MOP$-1404]#,##0.00",
	338: "#,##0.00\\ [$MVR-465]_-",
	339: "#,##0.00[$Nfk-873]",
	340: "[$NGN-466]#,##0.00",
	341: "[$NGN-467]#,##0.00",
	342: "[$NGN-469]#,##0.00",
	343: "[$NGN-471]#,##0.00",
	344: "[$NOK-103B]\\ #,##0.00",
	345: "[$NOK-183B]\\ #,##0.00",
	346: "[$NZ$-481]#,##0.00",
	347: "[$PKR-859]\\ #,##0.00",
	348: "[$PYG-474]#,##0.00",
	349: "[$Q-100A]#,##0.00",
	350: "[$R-436]\\ #,##0.00",
	351: "[$R-1C09]\\ #,##0.00",
	352: "[$R-435]\\ #,##0.00",
	353: "[$R$-416]\\ #,##0.00",
	354: "[$RD$-1C0A]#,##0.00",
	355: "#,##0.00\\ [$RF-487]",
	356: "[$RM-4409]#,##0.00",
	357: "[$RM-43E]#,##0.00",
	358: "#,##0.00\\ [$RON-418]",
	359: "[$Rp-421]#,##0.00",
	360: "[$Rs-420]#,##0.00_-",
	361: "[$Rs.-849]\\ #,##0.00",
	362: "#,##0.00\\ [$RSD-81A]",
	363: "#,##0.00\\ [$RSD-C1A]",
	364: "#,##0.00\\ [$RUB-46D]",
	365: "#,##0.00\\ [$RUB-444]",
	366: "[$S/.-C6B]\\ #,##0.00",
	367: "[$S/.-280A]\\ #,##0.00",
	368: "#,##0.00\\ [$SEK-143B]",
	369: "#,##0.00\\ [$SEK-1C3B]",
	370: "#,##0.00\\ [$so\u02BBm-443]",
	371: "#,##0.00\\ [$so\u02BBm-843]",
	372: "#,##0.00\\ [$SYP-45A]",
	373: "[$THB-41E]#,##0.00",
	374: "#,##0.00[$TMT-442]",
	375: "[$US$-3009]#,##0.00",
	376: "[$ZAR-46C]\\ #,##0.00",
	377: "[$ZAR-430]#,##0.00",
	378: "[$ZAR-431]#,##0.00",
	379: "[$ZAR-432]\\ #,##0.00",
	380: "[$ZAR-433]#,##0.00",
	381: "[$ZAR-434]\\ #,##0.00",
	382: "#,##0.00\\ [$z\u0142-415]",
	383: "#,##0.00\\ [$\u0434\u0435\u043D-42F]",
	384: "#,##0.00\\ [$КМ-201A]",
	385: "#,##0.00\\ [$КМ-1C1A]",
	386: "#,##0.00\\ [$\u043B\u0432.-402]",
	387: "#,##0.00\\ [$р.-423]",
	388: "#,##0.00\\ [$\u0441\u043E\u043C-440]",
	389: "#,##0.00\\ [$\u0441\u043E\u043C-428]",
	390: "[$\u062C.\u0645.-C01]\\ #,##0.00_-",
	391: "[$\u062F.\u0623.-2C01]\\ #,##0.00_-",
	392: "[$\u062F.\u0625.-3801]\\ #,##0.00_-",
	393: "[$\u062F.\u0628.-3C01]\\ #,##0.00_-",
	394: "[$\u062F.\u062A.-1C01]\\ #,##0.00_-",
	395: "[$\u062F.\u062C.-1401]\\ #,##0.00_-",
	396: "[$\u062F.\u0639.-801]\\ #,##0.00_-",
	397: "[$\u062F.\u0643.-3401]\\ #,##0.00_-",
	398: "[$\u062F.\u0644.-1001]#,##0.00_-",
	399: "[$\u062F.\u0645.-1801]\\ #,##0.00_-",
	400: "[$\u0631-846]\\ #,##0.00",
	401: "[$\u0631.\u0633.-401]\\ #,##0.00_-",
	402: "[$\u0631.\u0639.-2001]\\ #,##0.00_-",
	403: "[$\u0631.\u0642.-4001]\\ #,##0.00_-",
	404: "[$\u0631.\u064A.-2401]\\ #,##0.00_-",
	405: "[$\u0631\u06CC\u0627\u0644-429]#,##0.00_-",
	406: "[$\u0644.\u0633.-2801]\\ #,##0.00_-",
	407: "[$\u0644.\u0644.-3001]\\ #,##0.00_-",
	408: "[$\u1265\u122D-45E]#,##0.00",
	409: "[$\u0930\u0942-461]#,##0.00",
	410: "[$\u0DBB\u0DD4.-45B]\\ #,##0.00",
	411: "[$ADP]\\ #,##0.00",
	412: "[$AED]\\ #,##0.00",
	413: "[$AFA]\\ #,##0.00",
	414: "[$AFN]\\ #,##0.00",
	415: "[$ALL]\\ #,##0.00",
	416: "[$AMD]\\ #,##0.00",
	417: "[$ANG]\\ #,##0.00",
	418: "[$AOA]\\ #,##0.00",
	419: "[$ARS]\\ #,##0.00",
	420: "[$ATS]\\ #,##0.00",
	421: "[$AUD]\\ #,##0.00",
	422: "[$AWG]\\ #,##0.00",
	423: "[$AZM]\\ #,##0.00",
	424: "[$AZN]\\ #,##0.00",
	425: "[$BAM]\\ #,##0.00",
	426: "[$BBD]\\ #,##0.00",
	427: "[$BDT]\\ #,##0.00",
	428: "[$BEF]\\ #,##0.00",
	429: "[$BGL]\\ #,##0.00",
	430: "[$BGN]\\ #,##0.00",
	431: "[$BHD]\\ #,##0.00",
	432: "[$BIF]\\ #,##0.00",
	433: "[$BMD]\\ #,##0.00",
	434: "[$BND]\\ #,##0.00",
	435: "[$BOB]\\ #,##0.00",
	436: "[$BOV]\\ #,##0.00",
	437: "[$BRL]\\ #,##0.00",
	438: "[$BSD]\\ #,##0.00",
	439: "[$BTN]\\ #,##0.00",
	440: "[$BWP]\\ #,##0.00",
	441: "[$BYR]\\ #,##0.00",
	442: "[$BZD]\\ #,##0.00",
	443: "[$CAD]\\ #,##0.00",
	444: "[$CDF]\\ #,##0.00",
	445: "[$CHE]\\ #,##0.00",
	446: "[$CHF]\\ #,##0.00",
	447: "[$CHW]\\ #,##0.00",
	448: "[$CLF]\\ #,##0.00",
	449: "[$CLP]\\ #,##0.00",
	450: "[$CNY]\\ #,##0.00",
	451: "[$COP]\\ #,##0.00",
	452: "[$COU]\\ #,##0.00",
	453: "[$CRC]\\ #,##0.00",
	454: "[$CSD]\\ #,##0.00",
	455: "[$CUC]\\ #,##0.00",
	456: "[$CVE]\\ #,##0.00",
	457: "[$CYP]\\ #,##0.00",
	458: "[$CZK]\\ #,##0.00",
	459: "[$DEM]\\ #,##0.00",
	460: "[$DJF]\\ #,##0.00",
	461: "[$DKK]\\ #,##0.00",
	462: "[$DOP]\\ #,##0.00",
	463: "[$DZD]\\ #,##0.00",
	464: "[$ECS]\\ #,##0.00",
	465: "[$ECV]\\ #,##0.00",
	466: "[$EEK]\\ #,##0.00",
	467: "[$EGP]\\ #,##0.00",
	468: "[$ERN]\\ #,##0.00",
	469: "[$ESP]\\ #,##0.00",
	470: "[$ETB]\\ #,##0.00",
	471: "[$EUR]\\ #,##0.00",
	472: "[$FIM]\\ #,##0.00",
	473: "[$FJD]\\ #,##0.00",
	474: "[$FKP]\\ #,##0.00",
	475: "[$FRF]\\ #,##0.00",
	476: "[$GBP]\\ #,##0.00",
	477: "[$GEL]\\ #,##0.00",
	478: "[$GHC]\\ #,##0.00",
	479: "[$GHS]\\ #,##0.00",
	480: "[$GIP]\\ #,##0.00",
	481: "[$GMD]\\ #,##0.00",
	482: "[$GNF]\\ #,##0.00",
	483: "[$GRD]\\ #,##0.00",
	484: "[$GTQ]\\ #,##0.00",
	485: "[$GYD]\\ #,##0.00",
	486: "[$HKD]\\ #,##0.00",
	487: "[$HNL]\\ #,##0.00",
	488: "[$HRK]\\ #,##0.00",
	489: "[$HTG]\\ #,##0.00",
	490: "[$HUF]\\ #,##0.00",
	491: "[$IDR]\\ #,##0.00",
	492: "[$IEP]\\ #,##0.00",
	493: "[$ILS]\\ #,##0.00",
	494: "[$INR]\\ #,##0.00",
	495: "[$IQD]\\ #,##0.00",
	496: "[$IRR]\\ #,##0.00",
	497: "[$ISK]\\ #,##0.00",
	498: "[$ITL]\\ #,##0.00",
	499: "[$JMD]\\ #,##0.00",
	500: "[$JOD]\\ #,##0.00",
	501: "[$JPY]\\ #,##0.00",
	502: "[$KAF]\\ #,##0.00",
	503: "[$KES]\\ #,##0.00",
	504: "[$KGS]\\ #,##0.00",
	505: "[$KHR]\\ #,##0.00",
	506: "[$KMF]\\ #,##0.00",
	507: "[$KPW]\\ #,##0.00",
	508: "[$KRW]\\ #,##0.00",
	509: "[$KWD]\\ #,##0.00",
	510: "[$KYD]\\ #,##0.00",
	511: "[$KZT]\\ #,##0.00",
	512: "[$LAK]\\ #,##0.00",
	513: "[$LBP]\\ #,##0.00",
	514: "[$LKR]\\ #,##0.00",
	515: "[$LRD]\\ #,##0.00",
	516: "[$LSL]\\ #,##0.00",
	517: "[$LTL]\\ #,##0.00",
	518: "[$LUF]\\ #,##0.00",
	519: "[$LVL]\\ #,##0.00",
	520: "[$LYD]\\ #,##0.00",
	521: "[$MAD]\\ #,##0.00",
	522: "[$MDL]\\ #,##0.00",
	523: "[$MGA]\\ #,##0.00",
	524: "[$MGF]\\ #,##0.00",
	525: "[$MKD]\\ #,##0.00",
	526: "[$MMK]\\ #,##0.00",
	527: "[$MNT]\\ #,##0.00",
	528: "[$MOP]\\ #,##0.00",
	529: "[$MRO]\\ #,##0.00",
	530: "[$MTL]\\ #,##0.00",
	531: "[$MUR]\\ #,##0.00",
	532: "[$MVR]\\ #,##0.00",
	533: "[$MWK]\\ #,##0.00",
	534: "[$MXN]\\ #,##0.00",
	535: "[$MXV]\\ #,##0.00",
	536: "[$MYR]\\ #,##0.00",
	537: "[$MZM]\\ #,##0.00",
	538: "[$MZN]\\ #,##0.00",
	539: "[$NAD]\\ #,##0.00",
	540: "[$NGN]\\ #,##0.00",
	541: "[$NIO]\\ #,##0.00",
	542: "[$NLG]\\ #,##0.00",
	543: "[$NOK]\\ #,##0.00",
	544: "[$NPR]\\ #,##0.00",
	545: "[$NTD]\\ #,##0.00",
	546: "[$NZD]\\ #,##0.00",
	547: "[$OMR]\\ #,##0.00",
	548: "[$PAB]\\ #,##0.00",
	549: "[$PEN]\\ #,##0.00",
	550: "[$PGK]\\ #,##0.00",
	551: "[$PHP]\\ #,##0.00",
	552: "[$PKR]\\ #,##0.00",
	553: "[$PLN]\\ #,##0.00",
	554: "[$PTE]\\ #,##0.00",
	555: "[$PYG]\\ #,##0.00",
	556: "[$QAR]\\ #,##0.00",
	557: "[$ROL]\\ #,##0.00",
	558: "[$RON]\\ #,##0.00",
	559: "[$RSD]\\ #,##0.00",
	560: "[$RUB]\\ #,##0.00",
	561: "[$RUR]\\ #,##0.00",
	562: "[$RWF]\\ #,##0.00",
	563: "[$SAR]\\ #,##0.00",
	564: "[$SBD]\\ #,##0.00",
	565: "[$SCR]\\ #,##0.00",
	566: "[$SDD]\\ #,##0.00",
	567: "[$SDG]\\ #,##0.00",
	568: "[$SDP]\\ #,##0.00",
	569: "[$SEK]\\ #,##0.00",
	570: "[$SGD]\\ #,##0.00",
	571: "[$SHP]\\ #,##0.00",
	572: "[$SIT]\\ #,##0.00",
	573: "[$SKK]\\ #,##0.00",
	574: "[$SLL]\\ #,##0.00",
	575: "[$SOS]\\ #,##0.00",
	576: "[$SPL]\\ #,##0.00",
	577: "[$SRD]\\ #,##0.00",
	578: "[$SRG]\\ #,##0.00",
	579: "[$STD]\\ #,##0.00",
	580: "[$SVC]\\ #,##0.00",
	581: "[$SYP]\\ #,##0.00",
	582: "[$SZL]\\ #,##0.00",
	583: "[$THB]\\ #,##0.00",
	584: "[$TJR]\\ #,##0.00",
	585: "[$TJS]\\ #,##0.00",
	586: "[$TMM]\\ #,##0.00",
	587: "[$TMT]\\ #,##0.00",
	588: "[$TND]\\ #,##0.00",
	589: "[$TOP]\\ #,##0.00",
	590: "[$TRL]\\ #,##0.00",
	591: "[$TRY]\\ #,##0.00",
	592: "[$TTD]\\ #,##0.00",
	593: "[$TWD]\\ #,##0.00",
	594: "[$TZS]\\ #,##0.00",
	595: "[$UAH]\\ #,##0.00",
	596: "[$UGX]\\ #,##0.00",
	597: "[$USD]\\ #,##0.00",
	598: "[$USN]\\ #,##0.00",
	599: "[$USS]\\ #,##0.00",
	600: "[$UYI]\\ #,##0.00",
	601: "[$UYU]\\ #,##0.00",
	602: "[$UZS]\\ #,##0.00",
	603: "[$VEB]\\ #,##0.00",
	604: "[$VEF]\\ #,##0.00",
	605: "[$VND]\\ #,##0.00",
	606: "[$VUV]\\ #,##0.00",
	607: "[$WST]\\ #,##0.00",
	608: "[$XAF]\\ #,##0.00",
	609: "[$XAG]\\ #,##0.00",
	610: "[$XAU]\\ #,##0.00",
	611: "[$XB5]\\ #,##0.00",
	612: "[$XBA]\\ #,##0.00",
	613: "[$XBB]\\ #,##0.00",
	614: "[$XBC]\\ #,##0.00",
	615: "[$XBD]\\ #,##0.00",
	616: "[$XCD]\\ #,##0.00",
	617: "[$XDR]\\ #,##0.00",
	618: "[$XFO]\\ #,##0.00",
	619: "[$XFU]\\ #,##0.00",
	620: "[$XOF]\\ #,##0.00",
	621: "[$XPD]\\ #,##0.00",
	622: "[$XPF]\\ #,##0.00",
	623: "[$XPT]\\ #,##0.00",
	624: "[$XTS]\\ #,##0.00",
	625: "[$XXX]\\ #,##0.00",
	626: "[$YER]\\ #,##0.00",
	627: "[$YUM]\\ #,##0.00",
	628: "[$ZAR]\\ #,##0.00",
	629: "[$ZMK]\\ #,##0.00",
	630: "[$ZMW]\\ #,##0.00",
	631: "[$ZWD]\\ #,##0.00",
	632: "[$ZWL]\\ #,##0.00",
	633: "[$ZWN]\\ #,##0.00",
	634: "[$ZWR]\\ #,##0.00",
}

// builtInNumFmtFunc defined the format conversion functions map. Partial format
// code doesn't support currently and will return original string.
var builtInNumFmtFunc = map[int]func(i int, v string) string{
	0:  formatToString,
	1:  formatToInt,
	2:  formatToFloat,
	3:  formatToInt,
	4:  formatToFloat,
	9:  formatToC,
	10: formatToD,
	11: formatToE,
	12: formatToString, // Doesn't support currently
	13: formatToString, // Doesn't support currently
	14: parseTime,
	15: parseTime,
	16: parseTime,
	17: parseTime,
	18: parseTime,
	19: parseTime,
	20: parseTime,
	21: parseTime,
	22: parseTime,
	37: formatToA,
	38: formatToA,
	39: formatToB,
	40: formatToB,
	41: formatToString, // Doesn't support currently
	42: formatToString, // Doesn't support currently
	43: formatToString, // Doesn't support currently
	44: formatToString, // Doesn't support currently
	45: parseTime,
	46: parseTime,
	47: parseTime,
	48: formatToE,
	49: formatToString,
}

// validType defined the list of valid validation types.
var validType = map[string]string{
	"cell":          "cellIs",
	"date":          "date", // Doesn't support currently
	"time":          "time", // Doesn't support currently
	"average":       "aboveAverage",
	"duplicate":     "duplicateValues",
	"unique":        "uniqueValues",
	"top":           "top10",
	"bottom":        "top10",
	"text":          "text",              // Doesn't support currently
	"time_period":   "timePeriod",        // Doesn't support currently
	"blanks":        "containsBlanks",    // Doesn't support currently
	"no_blanks":     "notContainsBlanks", // Doesn't support currently
	"errors":        "containsErrors",    // Doesn't support currently
	"no_errors":     "notContainsErrors", // Doesn't support currently
	"2_color_scale": "2_color_scale",
	"3_color_scale": "3_color_scale",
	"data_bar":      "dataBar",
	"formula":       "expression",
}

// criteriaType defined the list of valid criteria types.
var criteriaType = map[string]string{
	"between":      "between",
	"not between":  "notBetween",
	"equal to":     "equal",
	"=":            "equal",
	"==":           "equal",
	"not equal to": "notEqual",
	"!=":           "notEqual",
	"<>":           "notEqual",
	"greater than": "greaterThan",
	">":            "greaterThan",
	"less than":    "lessThan",
	"<":            "lessThan",
	"greater than or equal to": "greaterThanOrEqual",
	">=": "greaterThanOrEqual",
	"less than or equal to": "lessThanOrEqual",
	"<=":             "lessThanOrEqual",
	"containing":     "containsText",
	"not containing": "notContains",
	"begins with":    "beginsWith",
	"ends with":      "endsWith",
	"yesterday":      "yesterday",
	"today":          "today",
	"last 7 days":    "last7Days",
	"last week":      "lastWeek",
	"this week":      "thisWeek",
	"continue week":  "continueWeek",
	"last month":     "lastMonth",
	"this month":     "thisMonth",
	"continue month": "continueMonth",
}

// formatToString provides function to return original string by given built-in
// number formats code and cell string.
func formatToString(i int, v string) string {
	return v
}

// formatToInt provides function to convert original string to integer format as
// string type by given built-in number formats code and cell string.
func formatToInt(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	return fmt.Sprintf("%d", int(f))
}

// formatToFloat provides function to convert original string to float format as
// string type by given built-in number formats code and cell string.
func formatToFloat(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	return fmt.Sprintf("%.2f", f)
}

// formatToA provides function to convert original string to special format as
// string type by given built-in number formats code and cell string.
func formatToA(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	if f < 0 {
		t := int(math.Abs(f))
		return fmt.Sprintf("(%d)", t)
	}
	t := int(f)
	return fmt.Sprintf("%d", t)
}

// formatToB provides function to convert original string to special format as
// string type by given built-in number formats code and cell string.
func formatToB(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	if f < 0 {
		return fmt.Sprintf("(%.2f)", f)
	}
	return fmt.Sprintf("%.2f", f)
}

// formatToC provides function to convert original string to special format as
// string type by given built-in number formats code and cell string.
func formatToC(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	f = f * 100
	return fmt.Sprintf("%d%%", int(f))
}

// formatToD provides function to convert original string to special format as
// string type by given built-in number formats code and cell string.
func formatToD(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	f = f * 100
	return fmt.Sprintf("%.2f%%", f)
}

// formatToE provides function to convert original string to special format as
// string type by given built-in number formats code and cell string.
func formatToE(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	return fmt.Sprintf("%.e", f)
}

// parseTime provides function to returns a string parsed using time.Time.
// Replace Excel placeholders with Go time placeholders. For example, replace
// yyyy with 2006. These are in a specific order, due to the fact that m is used
// in month, minute, and am/pm. It would be easier to fix that with regular
// expressions, but if it's possible to keep this simple it would be easier to
// maintain. Full-length month and days (e.g. March, Tuesday) have letters in
// them that would be replaced by other characters below (such as the 'h' in
// March, or the 'd' in Tuesday) below. First we convert them to arbitrary
// characters unused in Excel Date formats, and then at the end, turn them to
// what they should actually be.
// Based off: http://www.ozgrid.com/Excel/CustomFormats.htm
func parseTime(i int, v string) string {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return v
	}
	val := timeFromExcelTime(f, false)
	format := builtInNumFmt[i]

	replacements := []struct{ xltime, gotime string }{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"mmmm", "%%%%"},
		{"dddd", "&&&&"},
		{"dd", "02"},
		{"d", "2"},
		{"mmm", "Jan"},
		{"mmss", "0405"},
		{"ss", "05"},
		{"mm:", "04:"},
		{":mm", ":04"},
		{"mm", "01"},
		{"am/pm", "pm"},
		{"m/", "1/"},
		{"%%%%", "January"},
		{"&&&&", "Monday"},
	}
	// It is the presence of the "am/pm" indicator that determines if this is
	// a 12 hour or 24 hours time format, not the number of 'h' characters.
	if is12HourTime(format) {
		format = strings.Replace(format, "hh", "03", 1)
		format = strings.Replace(format, "h", "3", 1)
	} else {
		format = strings.Replace(format, "hh", "15", 1)
		format = strings.Replace(format, "h", "15", 1)
	}
	for _, repl := range replacements {
		format = strings.Replace(format, repl.xltime, repl.gotime, 1)
	}
	// If the hour is optional, strip it out, along with the possible dangling
	// colon that would remain.
	if val.Hour() < 1 {
		format = strings.Replace(format, "]:", "]", 1)
		format = strings.Replace(format, "[03]", "", 1)
		format = strings.Replace(format, "[3]", "", 1)
		format = strings.Replace(format, "[15]", "", 1)
	} else {
		format = strings.Replace(format, "[3]", "3", 1)
		format = strings.Replace(format, "[15]", "15", 1)
	}
	return val.Format(format)
}

// is12HourTime checks whether an Excel time format string is a 12 hours form.
func is12HourTime(format string) bool {
	return strings.Contains(format, "am/pm") || strings.Contains(format, "AM/PM") || strings.Contains(format, "a/p") || strings.Contains(format, "A/P")
}

// stylesReader provides function to get the pointer to the structure after
// deserialization of xl/styles.xml.
func (f *File) stylesReader() *xlsxStyleSheet {
	if f.Styles == nil {
		var styleSheet xlsxStyleSheet
		_ = xml.Unmarshal([]byte(f.readXML("xl/styles.xml")), &styleSheet)
		f.Styles = &styleSheet
	}
	return f.Styles
}

// styleSheetWriter provides function to save xl/styles.xml after serialize
// structure.
func (f *File) styleSheetWriter() {
	if f.Styles != nil {
		output, _ := xml.Marshal(f.Styles)
		f.saveFileList("xl/styles.xml", replaceWorkSheetsRelationshipsNameSpaceBytes(output))
	}
}

// parseFormatStyleSet provides function to parse the format settings of the
// cells and conditional formats.
func parseFormatStyleSet(style string) (*formatStyle, error) {
	format := formatStyle{
		DecimalPlaces: 2,
	}
	err := json.Unmarshal([]byte(style), &format)
	return &format, err
}

// NewStyle provides function to create style for cells by given style format.
// Note that the color field uses RGB color code.
//
// The following shows the border styles sorted by excelize index number:
//
//     Index | Name          | Weight | Style
//    -------+---------------+--------+-------------
//     0     | None          | 0      |
//     1     | Continuous    | 1      | -----------
//     2     | Continuous    | 2      | -----------
//     3     | Dash          | 1      | - - - - - -
//     4     | Dot           | 1      | . . . . . .
//     5     | Continuous    | 3      | -----------
//     6     | Double        | 3      | ===========
//     7     | Continuous    | 0      | -----------
//     8     | Dash          | 2      | - - - - - -
//     9     | Dash Dot      | 1      | - . - . - .
//     10    | Dash Dot      | 2      | - . - . - .
//     11    | Dash Dot Dot  | 1      | - . . - . .
//     12    | Dash Dot Dot  | 2      | - . . - . .
//     13    | SlantDash Dot | 2      | / - . / - .
//
// The following shows the borders in the order shown in the Excel dialog:
//
//     Index | Style       | Index | Style
//    -------+-------------+-------+-------------
//     0     | None        | 12    | - . . - . .
//     7     | ----------- | 13    | / - . / - .
//     4     | . . . . . . | 10    | - . - . - .
//     11    | - . . - . . | 8     | - - - - - -
//     9     | - . - . - . | 2     | -----------
//     3     | - - - - - - | 5     | -----------
//     1     | ----------- | 6     | ===========
//
// The following shows the shading styles sorted by excelize index number:
//
//     Index | Style           | Index | Style
//    -------+-----------------+-------+-----------------
//     0     | Horizontal      | 3     | Diagonal down
//     1     | Vertical        | 4     | From corner
//     2     | Diagonal Up     | 5     | From center
//
// The following shows the patterns styles sorted by excelize index number:
//
//     Index | Style           | Index | Style
//    -------+-----------------+-------+-----------------
//     0     | None            | 10    | darkTrellis
//     1     | solid           | 11    | lightHorizontal
//     2     | mediumGray      | 12    | lightVertical
//     3     | darkGray        | 13    | lightDown
//     4     | lightGray       | 14    | lightUp
//     5     | darkHorizontal  | 15    | lightGrid
//     6     | darkVertical    | 16    | lightTrellis
//     7     | darkDown        | 17    | gray125
//     8     | darkUp          | 18    | gray0625
//     9     | darkGrid        |       |
//
// The following the type of horizontal alignment in cells:
//
//     Style
//    ------------------
//     left
//     center
//     right
//     fill
//     justify
//     centerContinuous
//     distributed
//
// The following the type of vertical alignment in cells:
//
//     Style
//    ------------------
//     top
//     center
//     justify
//     distributed
//
// The following the type of font underline style:
//
//     Style
//    ------------------
//     single
//     double
//
// Excel's built-in all languages formats are shown in the following table:
//
//     Index | Format String
//    -------+----------------------------------------------------
//     0     | General
//     1     | 0
//     2     | 0.00
//     3     | #,##0
//     4     | #,##0.00
//     5     | ($#,##0_);($#,##0)
//     6     | ($#,##0_);[Red]($#,##0)
//     7     | ($#,##0.00_);($#,##0.00)
//     8     | ($#,##0.00_);[Red]($#,##0.00)
//     9     | 0%
//     10    | 0.00%
//     11    | 0.00E+00
//     12    | # ?/?
//     13    | # ??/??
//     14    | m/d/yy
//     15    | d-mmm-yy
//     16    | d-mmm
//     17    | mmm-yy
//     18    | h:mm AM/PM
//     19    | h:mm:ss AM/PM
//     20    | h:mm
//     21    | h:mm:ss
//     22    | m/d/yy h:mm
//     ...   | ...
//     37    | (#,##0_);(#,##0)
//     38    | (#,##0_);[Red](#,##0)
//     39    | (#,##0.00_);(#,##0.00)
//     40    | (#,##0.00_);[Red](#,##0.00)
//     41    | _(* #,##0_);_(* (#,##0);_(* "-"_);_(@_)
//     42    | _($* #,##0_);_($* (#,##0);_($* "-"_);_(@_)
//     43    | _(* #,##0.00_);_(* (#,##0.00);_(* "-"??_);_(@_)
//     44    | _($* #,##0.00_);_($* (#,##0.00);_($* "-"??_);_(@_)
//     45    | mm:ss
//     46    | [h]:mm:ss
//     47    | mm:ss.0
//     48    | ##0.0E+0
//     49    | @
//
// Number format code in zh-tw language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | [$-404]e/m/d
//     28    | [$-404]e"年"m"月"d"日"
//     29    | [$-404]e"年"m"月"d"日"
//     30    | m/d/yy
//     31    | yyyy"年"m"月"d"日"
//     32    | hh"時"mm"分"
//     33    | hh"時"mm"分"ss"秒"
//     34    | 上午/下午 hh"時"mm"分"
//     35    | 上午/下午 hh"時"mm"分"ss"秒"
//     36    | [$-404]e/m/d
//     50    | [$-404]e/m/d
//     51    | [$-404]e"年"m"月"d"日"
//     52    | 上午/下午 hh"時"mm"分"
//     53    | 上午/下午 hh"時"mm"分"ss"秒"
//     54    | [$-404]e"年"m"月"d"日"
//     55    | 上午/下午 hh"時"mm"分"
//     56    | 上午/下午 hh"時"mm"分"ss"秒"
//     57    | [$-404]e/m/d
//     58    | [$-404]e"年"m"月"d"日"
//
// Number format code in zh-cn language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | yyyy"年"m"月"
//     28    | m"月"d"日"
//     29    | m"月"d"日"
//     30    | m-d-yy
//     31    | yyyy"年"m"月"d"日"
//     32    | h"时"mm"分"
//     33    | h"时"mm"分"ss"秒"
//     34    | 上午/下午 h"时"mm"分"
//     35    | 上午/下午 h"时"mm"分"ss"秒
//     36    | yyyy"年"m"月
//     50    | yyyy"年"m"月
//     51    | m"月"d"日
//     52    | yyyy"年"m"月
//     53    | m"月"d"日
//     54    | m"月"d"日
//     55    | 上午/下午 h"时"mm"分
//     56    | 上午/下午 h"时"mm"分"ss"秒
//     57    | yyyy"年"m"月
//     58    | m"月"d"日"
//
// Number format code with unicode values provided for language glyphs where
// they occur in zh-tw language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | [$-404]e/m/
//     28    | [$-404]e"5E74"m"6708"d"65E5
//     29    | [$-404]e"5E74"m"6708"d"65E5
//     30    | m/d/y
//     31    | yyyy"5E74"m"6708"d"65E5
//     32    | hh"6642"mm"5206
//     33    | hh"6642"mm"5206"ss"79D2
//     34    | 4E0A5348/4E0B5348hh"6642"mm"5206
//     35    | 4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2
//     36    | [$-404]e/m/
//     50    | [$-404]e/m/
//     51    | [$-404]e"5E74"m"6708"d"65E5
//     52    | 4E0A5348/4E0B5348hh"6642"mm"5206
//     53    | 4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2
//     54    | [$-404]e"5E74"m"6708"d"65E5
//     55    | 4E0A5348/4E0B5348hh"6642"mm"5206
//     56    | 4E0A5348/4E0B5348hh"6642"mm"5206"ss"79D2
//     57    | [$-404]e/m/
//     58    | [$-404]e"5E74"m"6708"d"65E5"
//
// Number format code with unicode values provided for language glyphs where
// they occur in zh-cn language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | yyyy"5E74"m"6708
//     28    | m"6708"d"65E5
//     29    | m"6708"d"65E5
//     30    | m-d-y
//     31    | yyyy"5E74"m"6708"d"65E5
//     32    | h"65F6"mm"5206
//     33    | h"65F6"mm"5206"ss"79D2
//     34    | 4E0A5348/4E0B5348h"65F6"mm"5206
//     35    | 4E0A5348/4E0B5348h"65F6"mm"5206"ss"79D2
//     36    | yyyy"5E74"m"6708
//     50    | yyyy"5E74"m"6708
//     51    | m"6708"d"65E5
//     52    | yyyy"5E74"m"6708
//     53    | m"6708"d"65E5
//     54    | m"6708"d"65E5
//     55    | 4E0A5348/4E0B5348h"65F6"mm"5206
//     56    | 4E0A5348/4E0B5348h"65F6"mm"5206"ss"79D2
//     57    | yyyy"5E74"m"6708
//     58    | m"6708"d"65E5"
//
// Number format code in ja-jp language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | [$-411]ge.m.d
//     28    | [$-411]ggge"年"m"月"d"日
//     29    | [$-411]ggge"年"m"月"d"日
//     30    | m/d/y
//     31    | yyyy"年"m"月"d"日
//     32    | h"時"mm"分
//     33    | h"時"mm"分"ss"秒
//     34    | yyyy"年"m"月
//     35    | m"月"d"日
//     36    | [$-411]ge.m.d
//     50    | [$-411]ge.m.d
//     51    | [$-411]ggge"年"m"月"d"日
//     52    | yyyy"年"m"月
//     53    | m"月"d"日
//     54    | [$-411]ggge"年"m"月"d"日
//     55    | yyyy"年"m"月
//     56    | m"月"d"日
//     57    | [$-411]ge.m.d
//     58    | [$-411]ggge"年"m"月"d"日"
//
// Number format code in ko-kr language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | yyyy"年" mm"月" dd"日
//     28    | mm-d
//     29    | mm-d
//     30    | mm-dd-y
//     31    | yyyy"년" mm"월" dd"일
//     32    | h"시" mm"분
//     33    | h"시" mm"분" ss"초
//     34    | yyyy-mm-d
//     35    | yyyy-mm-d
//     36    | yyyy"年" mm"月" dd"日
//     50    | yyyy"年" mm"月" dd"日
//     51    | mm-d
//     52    | yyyy-mm-d
//     53    | yyyy-mm-d
//     54    | mm-d
//     55    | yyyy-mm-d
//     56    | yyyy-mm-d
//     57    | yyyy"年" mm"月" dd"日
//     58    | mm-dd
//
// Number format code with unicode values provided for language glyphs where
// they occur in ja-jp language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | [$-411]ge.m.d
//     28    | [$-411]ggge"5E74"m"6708"d"65E5
//     29    | [$-411]ggge"5E74"m"6708"d"65E5
//     30    | m/d/y
//     31    | yyyy"5E74"m"6708"d"65E5
//     32    | h"6642"mm"5206
//     33    | h"6642"mm"5206"ss"79D2
//     34    | yyyy"5E74"m"6708
//     35    | m"6708"d"65E5
//     36    | [$-411]ge.m.d
//     50    | [$-411]ge.m.d
//     51    | [$-411]ggge"5E74"m"6708"d"65E5
//     52    | yyyy"5E74"m"6708
//     53    | m"6708"d"65E5
//     54    | [$-411]ggge"5E74"m"6708"d"65E5
//     55    | yyyy"5E74"m"6708
//     56    | m"6708"d"65E5
//     57    | [$-411]ge.m.d
//     58    | [$-411]ggge"5E74"m"6708"d"65E5"
//
// Number format code with unicode values provided for language glyphs where
// they occur in ko-kr language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     27    | yyyy"5E74" mm"6708" dd"65E5
//     28    | mm-d
//     29    | mm-d
//     30    | mm-dd-y
//     31    | yyyy"B144" mm"C6D4" dd"C77C
//     32    | h"C2DC" mm"BD84
//     33    | h"C2DC" mm"BD84" ss"CD08
//     34    | yyyy-mm-d
//     35    | yyyy-mm-d
//     36    | yyyy"5E74" mm"6708" dd"65E5
//     50    | yyyy"5E74" mm"6708" dd"65E5
//     51    | mm-d
//     52    | yyyy-mm-d
//     53    | yyyy-mm-d
//     54    | mm-d
//     55    | yyyy-mm-d
//     56    | yyyy-mm-d
//     57    | yyyy"5E74" mm"6708" dd"65E5
//     58    | mm-dd
//
// Number format code in th-th language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     59    | t
//     60    | t0.0
//     61    | t#,##
//     62    | t#,##0.0
//     67    | t0
//     68    | t0.00
//     69    | t# ?/
//     70    | t# ??/?
//     71    | ว/ด/ปปป
//     72    | ว-ดดด-ป
//     73    | ว-ดด
//     74    | ดดด-ป
//     75    | ช:น
//     76    | ช:นน:ท
//     77    | ว/ด/ปปปป ช:น
//     78    | นน:ท
//     79    | [ช]:นน:ท
//     80    | นน:ทท.
//     81    | d/m/bb
//
// Number format code with unicode values provided for language glyphs where
// they occur in th-th language:
//
//     Index | Symbol
//    -------+-------------------------------------------
//     59    | t
//     60    | t0.0
//     61    | t#,##
//     62    | t#,##0.0
//     67    | t0
//     68    | t0.00
//     69    | t# ?/
//     70    | t# ??/?
//     71    | 0E27/0E14/0E1B0E1B0E1B0E1
//     72    | 0E27-0E140E140E14-0E1B0E1
//     73    | 0E27-0E140E140E1
//     74    | 0E140E140E14-0E1B0E1
//     75    | 0E0A:0E190E1
//     76    | 0E0A:0E190E19:0E170E1
//     77    | 0E27/0E14/0E1B0E1B0E1B0E1B 0E0A:0E190E1
//     78    | 0E190E19:0E170E1
//     79    | [0E0A]:0E190E19:0E170E1
//     80    | 0E190E19:0E170E17.
//     81    | d/m/bb
//
// Excelize built-in currency formats are shown in the following table, only
// support these types in the following table (Index number is used only for
// markup and is not used inside an Excel file and you can't get formatted value
// by the function GetCellValue) currently:
//
//     Index | Symbol
//    -------+---------------------------------------------------------------
//     164   | CN¥
//     165   | $ English (China)
//     166   | $ Cherokee (United States)
//     167   | $ Chinese (Singapore)
//     168   | $ Chinese (Taiwan)
//     169   | $ English (Australia)
//     170   | $ English (Belize)
//     171   | $ English (Canada)
//     172   | $ English (Jamaica)
//     173   | $ English (New Zealand)
//     174   | $ English (Singapore)
//     175   | $ English (Trinidad & Tobago)
//     176   | $ English (U.S. Vigin Islands)
//     177   | $ English (United States)
//     178   | $ French (Canada)
//     179   | $ Hawaiian (United States)
//     180   | $ Malay (Brunei)
//     181   | $ Quechua (Ecuador)
//     182   | $ Spanish (Chile)
//     183   | $ Spanish (Colombia)
//     184   | $ Spanish (Ecuador)
//     185   | $ Spanish (El Salvador)
//     186   | $ Spanish (Mexico)
//     187   | $ Spanish (Puerto Rico)
//     188   | $ Spanish (United States)
//     189   | $ Spanish (Uruguay)
//     190   | £ English (United Kingdom)
//     191   | £ Scottish Gaelic (United Kingdom)
//     192   | £ Welsh (United Kindom)
//     193   | ¥ Chinese (China)
//     194   | ¥ Japanese (Japan)
//     195   | ¥ Sichuan Yi (China)
//     196   | ¥ Tibetan (China)
//     197   | ¥ Uyghur (China)
//     198   | ֏ Armenian (Armenia)
//     199   | ؋ Pashto (Afghanistan)
//     200   | ؋ Persian (Afghanistan)
//     201   | ৳ Bengali (Bangladesh)
//     202   | ៛ Khmer (Cambodia)
//     203   | ₡ Spanish (Costa Rica)
//     204   | ₦ Hausa (Nigeria)
//     205   | ₦ Igbo (Nigeria)
//     206   | ₦ Yoruba (Nigeria)
//     207   | ₩ Korean (South Korea)
//     208   | ₪ Hebrew (Israel)
//     209   | ₫ Vietnamese (Vietnam)
//     210   | € Basque (Spain)
//     211   | € Breton (France)
//     212   | € Catalan (Spain)
//     213   | € Corsican (France)
//     214   | € Dutch (Belgium)
//     215   | € Dutch (Netherlands)
//     216   | € English (Ireland)
//     217   | € Estonian (Estonia)
//     218   | € Euro (€ 123)
//     219   | € Euro (123 €)
//     220   | € Finnish (Finland)
//     221   | € French (Belgium)
//     222   | € French (France)
//     223   | € French (Luxembourg)
//     224   | € French (Monaco)
//     225   | € French (Réunion)
//     226   | € Galician (Spain)
//     227   | € German (Austria)
//     228   | € German (Luxembourg)
//     229   | € Greek (Greece)
//     230   | € Inari Sami (Finland)
//     231   | € Irish (Ireland)
//     232   | € Italian (Italy)
//     233   | € Latin (Italy)
//     234   | € Latin, Serbian (Montenegro)
//     235   | € Larvian (Latvia)
//     236   | € Lithuanian (Lithuania)
//     237   | € Lower Sorbian (Germany)
//     238   | € Luxembourgish (Luxembourg)
//     239   | € Maltese (Malta)
//     240   | € Northern Sami (Finland)
//     241   | € Occitan (France)
//     242   | € Portuguese (Portugal)
//     243   | € Serbian (Montenegro)
//     244   | € Skolt Sami (Finland)
//     245   | € Slovak (Slovakia)
//     246   | € Slovenian (Slovenia)
//     247   | € Spanish (Spain)
//     248   | € Swedish (Finland)
//     249   | € Swiss German (France)
//     250   | € Upper Sorbian (Germany)
//     251   | € Western Frisian (Netherlands)
//     252   | ₭ Lao (Laos)
//     253   | ₮ Mongolian (Mongolia)
//     254   | ₮ Mongolian, Mongolian (Mongolia)
//     255   | ₱ English (Philippines)
//     256   | ₱ Filipino (Philippines)
//     257   | ₴ Ukrainian (Ukraine)
//     258   | ₸ Kazakh (Kazakhstan)
//     259   | ₹ Arabic, Kashmiri (India)
//     260   | ₹ English (India)
//     261   | ₹ Gujarati (India)
//     262   | ₹ Hindi (India)
//     263   | ₹ Kannada (India)
//     264   | ₹ Kashmiri (India)
//     265   | ₹ Konkani (India)
//     266   | ₹ Manipuri (India)
//     267   | ₹ Marathi (India)
//     268   | ₹ Nepali (India)
//     269   | ₹ Oriya (India)
//     270   | ₹ Punjabi (India)
//     271   | ₹ Sanskrit (India)
//     272   | ₹ Sindhi (India)
//     273   | ₹ Tamil (India)
//     274   | ₹ Urdu (India)
//     275   | ₺ Turkish (Turkey)
//     276   | ₼ Azerbaijani (Azerbaijan)
//     277   | ₼ Cyrillic, Azerbaijani (Azerbaijan)
//     278   | ₽ Russian (Russia)
//     279   | ₽ Sakha (Russia)
//     280   | ₾ Georgian (Georgia)
//     281   | B/. Spanish (Panama)
//     282   | Br Oromo (Ethiopia)
//     283   | Br Somali (Ethiopia)
//     284   | Br Tigrinya (Ethiopia)
//     285   | Bs Quechua (Bolivia)
//     286   | Bs Spanish (Bolivia)
//     287   | BS. Spanish (Venezuela)
//     288   | BWP Tswana (Botswana)
//     289   | C$ Spanish (Nicaragua)
//     290   | CA$ Latin, Inuktitut (Canada)
//     291   | CA$ Mohawk (Canada)
//     292   | CA$ Unified Canadian Aboriginal Syllabics, Inuktitut (Canada)
//     293   | CFA French (Mali)
//     294   | CFA French (Senegal)
//     295   | CFA Fulah (Senegal)
//     296   | CFA Wolof (Senegal)
//     297   | CHF French (Switzerland)
//     298   | CHF German (Liechtenstein)
//     299   | CHF German (Switzerland)
//     300   | CHF Italian (Switzerland)
//     301   | CHF Romansh (Switzerland)
//     302   | CLP Mapuche (Chile)
//     303   | CN¥ Mongolian, Mongolian (China)
//     304   | DZD Central Atlas Tamazight (Algeria)
//     305   | FCFA French (Cameroon)
//     306   | Ft Hungarian (Hungary)
//     307   | G French (Haiti)
//     308   | Gs. Spanish (Paraguay)
//     309   | GTQ K'iche' (Guatemala)
//     310   | HK$ Chinese (Hong Kong (China))
//     311   | HK$ English (Hong Kong (China))
//     312   | HRK Croatian (Croatia)
//     313   | IDR English (Indonesia)
//     314   | IQD Arbic, Central Kurdish (Iraq)
//     315   | ISK Icelandic (Iceland)
//     316   | K Burmese (Myanmar (Burma))
//     317   | Kč Czech (Czech Republic)
//     318   | KM Bosnian (Bosnia & Herzegovina)
//     319   | KM Croatian (Bosnia & Herzegovina)
//     320   | KM Latin, Serbian (Bosnia & Herzegovina)
//     321   | kr Faroese (Faroe Islands)
//     322   | kr Northern Sami (Norway)
//     323   | kr Northern Sami (Sweden)
//     324   | kr Norwegian Bokmål (Norway)
//     325   | kr Norwegian Nynorsk (Norway)
//     326   | kr Swedish (Sweden)
//     327   | kr. Danish (Denmark)
//     328   | kr. Kalaallisut (Greenland)
//     329   | Ksh Swahili (kenya)
//     330   | L Romanian (Moldova)
//     331   | L Russian (Moldova)
//     332   | L Spanish (Honduras)
//     333   | Lekë Albanian (Albania)
//     334   | MAD Arabic, Central Atlas Tamazight (Morocco)
//     335   | MAD French (Morocco)
//     336   | MAD Tifinagh, Central Atlas Tamazight (Morocco)
//     337   | MOP$ Chinese (Macau (China))
//     338   | MVR Divehi (Maldives)
//     339   | Nfk Tigrinya (Eritrea)
//     340   | NGN Bini (Nigeria)
//     341   | NGN Fulah (Nigeria)
//     342   | NGN Ibibio (Nigeria)
//     343   | NGN Kanuri (Nigeria)
//     344   | NOK Lule Sami (Norway)
//     345   | NOK Southern Sami (Norway)
//     346   | NZ$ Maori (New Zealand)
//     347   | PKR Sindhi (Pakistan)
//     348   | PYG Guarani (Paraguay)
//     349   | Q Spanish (Guatemala)
//     350   | R Afrikaans (South Africa)
//     351   | R English (South Africa)
//     352   | R Zulu (South Africa)
//     353   | R$ Portuguese (Brazil)
//     354   | RD$ Spanish (Dominican Republic)
//     355   | RF Kinyarwanda (Rwanda)
//     356   | RM English (Malaysia)
//     357   | RM Malay (Malaysia)
//     358   | RON Romanian (Romania)
//     359   | Rp Indonesoan (Indonesia)
//     360   | Rs Urdu (Pakistan)
//     361   | Rs. Tamil (Sri Lanka)
//     362   | RSD Latin, Serbian (Serbia)
//     363   | RSD Serbian (Serbia)
//     364   | RUB Bashkir (Russia)
//     365   | RUB Tatar (Russia)
//     366   | S/. Quechua (Peru)
//     367   | S/. Spanish (Peru)
//     368   | SEK Lule Sami (Sweden)
//     369   | SEK Southern Sami (Sweden)
//     370   | soʻm Latin, Uzbek (Uzbekistan)
//     371   | soʻm Uzbek (Uzbekistan)
//     372   | SYP Syriac (Syria)
//     373   | THB Thai (Thailand)
//     374   | TMT Turkmen (Turkmenistan)
//     375   | US$ English (Zimbabwe)
//     376   | ZAR Northern Sotho (South Africa)
//     377   | ZAR Southern Sotho (South Africa)
//     378   | ZAR Tsonga (South Africa)
//     379   | ZAR Tswana (south Africa)
//     380   | ZAR Venda (South Africa)
//     381   | ZAR Xhosa (South Africa)
//     382   | zł Polish (Poland)
//     383   | ден Macedonian (Macedonia)
//     384   | KM Cyrillic, Bosnian (Bosnia & Herzegovina)
//     385   | KM Serbian (Bosnia & Herzegovina)
//     386   | лв. Bulgarian (Bulgaria)
//     387   | p. Belarusian (Belarus)
//     388   | сом Kyrgyz (Kyrgyzstan)
//     389   | сом Tajik (Tajikistan)
//     390   | ج.م. Arabic (Egypt)
//     391   | د.أ. Arabic (Jordan)
//     392   | د.أ. Arabic (United Arab Emirates)
//     393   | د.ب. Arabic (Bahrain)
//     394   | د.ت. Arabic (Tunisia)
//     395   | د.ج. Arabic (Algeria)
//     396   | د.ع. Arabic (Iraq)
//     397   | د.ك. Arabic (Kuwait)
//     398   | د.ل. Arabic (Libya)
//     399   | د.م. Arabic (Morocco)
//     400   | ر Punjabi (Pakistan)
//     401   | ر.س. Arabic (Saudi Arabia)
//     402   | ر.ع. Arabic (Oman)
//     403   | ر.ق. Arabic (Qatar)
//     404   | ر.ي. Arabic (Yemen)
//     405   | ریال Persian (Iran)
//     406   | ل.س. Arabic (Syria)
//     407   | ل.ل. Arabic (Lebanon)
//     408   | ብር Amharic (Ethiopia)
//     409   | रू Nepaol (Nepal)
//     410   | රු. Sinhala (Sri Lanka)
//     411   | ADP
//     412   | AED
//     413   | AFA
//     414   | AFN
//     415   | ALL
//     416   | AMD
//     417   | ANG
//     418   | AOA
//     419   | ARS
//     420   | ATS
//     421   | AUD
//     422   | AWG
//     423   | AZM
//     424   | AZN
//     425   | BAM
//     426   | BBD
//     427   | BDT
//     428   | BEF
//     429   | BGL
//     430   | BGN
//     431   | BHD
//     432   | BIF
//     433   | BMD
//     434   | BND
//     435   | BOB
//     436   | BOV
//     437   | BRL
//     438   | BSD
//     439   | BTN
//     440   | BWP
//     441   | BYR
//     442   | BZD
//     443   | CAD
//     444   | CDF
//     445   | CHE
//     446   | CHF
//     447   | CHW
//     448   | CLF
//     449   | CLP
//     450   | CNY
//     451   | COP
//     452   | COU
//     453   | CRC
//     454   | CSD
//     455   | CUC
//     456   | CVE
//     457   | CYP
//     458   | CZK
//     459   | DEM
//     460   | DJF
//     461   | DKK
//     462   | DOP
//     463   | DZD
//     464   | ECS
//     465   | ECV
//     466   | EEK
//     467   | EGP
//     468   | ERN
//     469   | ESP
//     470   | ETB
//     471   | EUR
//     472   | FIM
//     473   | FJD
//     474   | FKP
//     475   | FRF
//     476   | GBP
//     477   | GEL
//     478   | GHC
//     479   | GHS
//     480   | GIP
//     481   | GMD
//     482   | GNF
//     483   | GRD
//     484   | GTQ
//     485   | GYD
//     486   | HKD
//     487   | HNL
//     488   | HRK
//     489   | HTG
//     490   | HUF
//     491   | IDR
//     492   | IEP
//     493   | ILS
//     494   | INR
//     495   | IQD
//     496   | IRR
//     497   | ISK
//     498   | ITL
//     499   | JMD
//     500   | JOD
//     501   | JPY
//     502   | KAF
//     503   | KES
//     504   | KGS
//     505   | KHR
//     506   | KMF
//     507   | KPW
//     508   | KRW
//     509   | KWD
//     510   | KYD
//     511   | KZT
//     512   | LAK
//     513   | LBP
//     514   | LKR
//     515   | LRD
//     516   | LSL
//     517   | LTL
//     518   | LUF
//     519   | LVL
//     520   | LYD
//     521   | MAD
//     522   | MDL
//     523   | MGA
//     524   | MGF
//     525   | MKD
//     526   | MMK
//     527   | MNT
//     528   | MOP
//     529   | MRO
//     530   | MTL
//     531   | MUR
//     532   | MVR
//     533   | MWK
//     534   | MXN
//     535   | MXV
//     536   | MYR
//     537   | MZM
//     538   | MZN
//     539   | NAD
//     540   | NGN
//     541   | NIO
//     542   | NLG
//     543   | NOK
//     544   | NPR
//     545   | NTD
//     546   | NZD
//     547   | OMR
//     548   | PAB
//     549   | PEN
//     550   | PGK
//     551   | PHP
//     552   | PKR
//     553   | PLN
//     554   | PTE
//     555   | PYG
//     556   | QAR
//     557   | ROL
//     558   | RON
//     559   | RSD
//     560   | RUB
//     561   | RUR
//     562   | RWF
//     563   | SAR
//     564   | SBD
//     565   | SCR
//     566   | SDD
//     567   | SDG
//     568   | SDP
//     569   | SEK
//     570   | SGD
//     571   | SHP
//     572   | SIT
//     573   | SKK
//     574   | SLL
//     575   | SOS
//     576   | SPL
//     577   | SRD
//     578   | SRG
//     579   | STD
//     580   | SVC
//     581   | SYP
//     582   | SZL
//     583   | THB
//     584   | TJR
//     585   | TJS
//     586   | TMM
//     587   | TMT
//     588   | TND
//     589   | TOP
//     590   | TRL
//     591   | TRY
//     592   | TTD
//     593   | TWD
//     594   | TZS
//     595   | UAH
//     596   | UGX
//     597   | USD
//     598   | USN
//     599   | USS
//     600   | UYI
//     601   | UYU
//     602   | UZS
//     603   | VEB
//     604   | VEF
//     605   | VND
//     606   | VUV
//     607   | WST
//     608   | XAF
//     609   | XAG
//     610   | XAU
//     611   | XB5
//     612   | XBA
//     613   | XBB
//     614   | XBC
//     615   | XBD
//     616   | XCD
//     617   | XDR
//     618   | XFO
//     619   | XFU
//     620   | XOF
//     621   | XPD
//     622   | XPF
//     623   | XPT
//     624   | XTS
//     625   | XXX
//     626   | YER
//     627   | YUM
//     628   | ZAR
//     629   | ZMK
//     630   | ZMW
//     631   | ZWD
//     632   | ZWL
//     633   | ZWN
//     634   | ZWR
//
// Excelize support set custom number format for cell. For example, set number
// as date type in Uruguay (Spanish) format for Sheet1!A6:
//
//    xlsx := excelize.NewFile()
//    xlsx.SetCellValue("Sheet1", "A6", 42920.5)
//    style, _ := xlsx.NewStyle(`{"custom_number_format": "[$-380A]dddd\\,\\ dd\" de \"mmmm\" de \"yyyy;@"}`)
//    xlsx.SetCellStyle("Sheet1", "A6", "A6", style)
//
// Cell Sheet1!A6 in the Excel Application: martes, 04 de Julio de 2017
//
func (f *File) NewStyle(style string) (int, error) {
	var cellXfsID, fontID, borderID, fillID int
	s := f.stylesReader()
	fs, err := parseFormatStyleSet(style)
	if err != nil {
		return cellXfsID, err
	}
	numFmtID := setNumFmt(s, fs)

	if fs.Font != nil {
		font, _ := xml.Marshal(setFont(fs))
		s.Fonts.Count++
		s.Fonts.Font = append(s.Fonts.Font, &xlsxFont{
			Font: string(font[6 : len(font)-7]),
		})
		fontID = s.Fonts.Count - 1
	}

	s.Borders.Count++
	s.Borders.Border = append(s.Borders.Border, setBorders(fs))
	borderID = s.Borders.Count - 1

	s.Fills.Count++
	s.Fills.Fill = append(s.Fills.Fill, setFills(fs, true))
	fillID = s.Fills.Count - 1

	applyAlignment, alignment := fs.Alignment != nil, setAlignment(fs)
	applyProtection, protection := fs.Protection != nil, setProtection(fs)
	cellXfsID = setCellXfs(s, fontID, numFmtID, fillID, borderID, applyAlignment, applyProtection, alignment, protection)
	return cellXfsID, nil
}

// NewConditionalStyle provides function to create style for conditional format
// by given style format. The parameters are the same as function NewStyle().
// Note that the color field uses RGB color code and only support to set font,
// fills, alignment and borders currently.
func (f *File) NewConditionalStyle(style string) (int, error) {
	s := f.stylesReader()
	fs, err := parseFormatStyleSet(style)
	if err != nil {
		return 0, err
	}
	dxf := dxf{
		Fill:      setFills(fs, false),
		Alignment: setAlignment(fs),
		Border:    setBorders(fs),
	}
	if fs.Font != nil {
		dxf.Font = setFont(fs)
	}
	dxfStr, _ := xml.Marshal(dxf)
	if s.Dxfs == nil {
		s.Dxfs = &xlsxDxfs{}
	}
	s.Dxfs.Count++
	s.Dxfs.Dxfs = append(s.Dxfs.Dxfs, &xlsxDxf{
		Dxf: string(dxfStr[5 : len(dxfStr)-6]),
	})
	return s.Dxfs.Count - 1, nil
}

// setFont provides function to add font style by given cell format settings.
func setFont(formatStyle *formatStyle) *font {
	fontUnderlineType := map[string]string{"single": "single", "double": "double"}
	if formatStyle.Font.Size < 1 {
		formatStyle.Font.Size = 11
	}
	if formatStyle.Font.Color == "" {
		formatStyle.Font.Color = "#000000"
	}
	f := font{
		B:      formatStyle.Font.Bold,
		I:      formatStyle.Font.Italic,
		Sz:     &attrValInt{Val: formatStyle.Font.Size},
		Color:  &xlsxColor{RGB: getPaletteColor(formatStyle.Font.Color)},
		Name:   &attrValString{Val: formatStyle.Font.Family},
		Family: &attrValInt{Val: 2},
	}
	if f.Name.Val == "" {
		f.Name.Val = "Calibri"
		f.Scheme = &attrValString{Val: "minor"}
	}
	val, ok := fontUnderlineType[formatStyle.Font.Underline]
	if ok {
		f.U = &attrValString{Val: val}
	}
	return &f
}

// setNumFmt provides function to check if number format code in the range of
// built-in values.
func setNumFmt(style *xlsxStyleSheet, formatStyle *formatStyle) int {
	dp := "0."
	numFmtID := 164 // Default custom number format code from 164.
	if formatStyle.DecimalPlaces < 0 || formatStyle.DecimalPlaces > 30 {
		formatStyle.DecimalPlaces = 2
	}
	for i := 0; i < formatStyle.DecimalPlaces; i++ {
		dp += "0"
	}
	if formatStyle.CustomNumFmt != nil {
		return setCustomNumFmt(style, formatStyle)
	}
	_, ok := builtInNumFmt[formatStyle.NumFmt]
	if !ok {
		fc, currency := currencyNumFmt[formatStyle.NumFmt]
		if !currency {
			return setLangNumFmt(style, formatStyle)
		}
		fc = strings.Replace(fc, "0.00", dp, -1)
		if formatStyle.NegRed {
			fc = fc + ";[Red]" + fc
		}
		if style.NumFmts != nil {
			numFmtID = style.NumFmts.NumFmt[len(style.NumFmts.NumFmt)-1].NumFmtID + 1
			nf := xlsxNumFmt{
				FormatCode: fc,
				NumFmtID:   numFmtID,
			}
			style.NumFmts.NumFmt = append(style.NumFmts.NumFmt, &nf)
			style.NumFmts.Count++
		} else {
			nf := xlsxNumFmt{
				FormatCode: fc,
				NumFmtID:   numFmtID,
			}
			numFmts := xlsxNumFmts{
				NumFmt: []*xlsxNumFmt{&nf},
				Count:  1,
			}
			style.NumFmts = &numFmts
		}
		return numFmtID
	}
	return formatStyle.NumFmt
}

// setCustomNumFmt provides function to set custom number format code.
func setCustomNumFmt(style *xlsxStyleSheet, formatStyle *formatStyle) int {
	nf := xlsxNumFmt{FormatCode: *formatStyle.CustomNumFmt}
	if style.NumFmts != nil {
		nf.NumFmtID = style.NumFmts.NumFmt[len(style.NumFmts.NumFmt)-1].NumFmtID + 1
		style.NumFmts.NumFmt = append(style.NumFmts.NumFmt, &nf)
		style.NumFmts.Count++
	} else {
		nf.NumFmtID = 164
		numFmts := xlsxNumFmts{
			NumFmt: []*xlsxNumFmt{&nf},
			Count:  1,
		}
		style.NumFmts = &numFmts
	}
	return nf.NumFmtID
}

// setLangNumFmt provides function to set number format code with language.
func setLangNumFmt(style *xlsxStyleSheet, formatStyle *formatStyle) int {
	numFmts, ok := langNumFmt[formatStyle.Lang]
	if !ok {
		return 0
	}
	var fc string
	fc, ok = numFmts[formatStyle.NumFmt]
	if !ok {
		return 0
	}
	nf := xlsxNumFmt{FormatCode: fc}
	if style.NumFmts != nil {
		nf.NumFmtID = style.NumFmts.NumFmt[len(style.NumFmts.NumFmt)-1].NumFmtID + 1
		style.NumFmts.NumFmt = append(style.NumFmts.NumFmt, &nf)
		style.NumFmts.Count++
	} else {
		nf.NumFmtID = formatStyle.NumFmt
		numFmts := xlsxNumFmts{
			NumFmt: []*xlsxNumFmt{&nf},
			Count:  1,
		}
		style.NumFmts = &numFmts
	}
	return nf.NumFmtID
}

// setFills provides function to add fill elements in the styles.xml by given
// cell format settings.
func setFills(formatStyle *formatStyle, fg bool) *xlsxFill {
	var patterns = []string{
		"none",
		"solid",
		"mediumGray",
		"darkGray",
		"lightGray",
		"darkHorizontal",
		"darkVertical",
		"darkDown",
		"darkUp",
		"darkGrid",
		"darkTrellis",
		"lightHorizontal",
		"lightVertical",
		"lightDown",
		"lightUp",
		"lightGrid",
		"lightTrellis",
		"gray125",
		"gray0625",
	}

	var variants = []float64{
		90,
		0,
		45,
		135,
	}

	var fill xlsxFill
	switch formatStyle.Fill.Type {
	case "gradient":
		if len(formatStyle.Fill.Color) != 2 {
			break
		}
		var gradient xlsxGradientFill
		switch formatStyle.Fill.Shading {
		case 0, 1, 2, 3:
			gradient.Degree = variants[formatStyle.Fill.Shading]
		case 4:
			gradient.Type = "path"
		case 5:
			gradient.Type = "path"
			gradient.Bottom = 0.5
			gradient.Left = 0.5
			gradient.Right = 0.5
			gradient.Top = 0.5
		default:
			break
		}
		var stops []*xlsxGradientFillStop
		for index, color := range formatStyle.Fill.Color {
			var stop xlsxGradientFillStop
			stop.Position = float64(index)
			stop.Color.RGB = getPaletteColor(color)
			stops = append(stops, &stop)
		}
		gradient.Stop = stops
		fill.GradientFill = &gradient
	case "pattern":
		if formatStyle.Fill.Pattern > 18 || formatStyle.Fill.Pattern < 0 {
			break
		}
		if len(formatStyle.Fill.Color) < 1 {
			break
		}
		var pattern xlsxPatternFill
		pattern.PatternType = patterns[formatStyle.Fill.Pattern]
		if fg {
			pattern.FgColor.RGB = getPaletteColor(formatStyle.Fill.Color[0])
		} else {
			pattern.BgColor.RGB = getPaletteColor(formatStyle.Fill.Color[0])
		}
		fill.PatternFill = &pattern
	}
	return &fill
}

// setAlignment provides function to formatting information pertaining to text
// alignment in cells. There are a variety of choices for how text is aligned
// both horizontally and vertically, as well as indentation settings, and so on.
func setAlignment(formatStyle *formatStyle) *xlsxAlignment {
	var alignment xlsxAlignment
	if formatStyle.Alignment != nil {
		alignment.Horizontal = formatStyle.Alignment.Horizontal
		alignment.Indent = formatStyle.Alignment.Indent
		alignment.JustifyLastLine = formatStyle.Alignment.JustifyLastLine
		alignment.ReadingOrder = formatStyle.Alignment.ReadingOrder
		alignment.RelativeIndent = formatStyle.Alignment.RelativeIndent
		alignment.ShrinkToFit = formatStyle.Alignment.ShrinkToFit
		alignment.TextRotation = formatStyle.Alignment.TextRotation
		alignment.Vertical = formatStyle.Alignment.Vertical
		alignment.WrapText = formatStyle.Alignment.WrapText
	}
	return &alignment
}

// setProtection provides function to set protection properties associated
// with the cell.
func setProtection(formatStyle *formatStyle) *xlsxProtection {
	var protection xlsxProtection
	if formatStyle.Protection != nil {
		protection.Hidden = formatStyle.Protection.Hidden
		protection.Locked = formatStyle.Protection.Locked
	}
	return &protection
}

// setBorders provides function to add border elements in the styles.xml by
// given borders format settings.
func setBorders(formatStyle *formatStyle) *xlsxBorder {
	var styles = []string{
		"none",
		"thin",
		"medium",
		"dashed",
		"dotted",
		"thick",
		"double",
		"hair",
		"mediumDashed",
		"dashDot",
		"mediumDashDot",
		"dashDotDot",
		"mediumDashDotDot",
		"slantDashDot",
	}

	var border xlsxBorder
	for _, v := range formatStyle.Border {
		if 0 <= v.Style && v.Style < 14 {
			var color xlsxColor
			color.RGB = getPaletteColor(v.Color)
			switch v.Type {
			case "left":
				border.Left.Style = styles[v.Style]
				border.Left.Color = &color
			case "right":
				border.Right.Style = styles[v.Style]
				border.Right.Color = &color
			case "top":
				border.Top.Style = styles[v.Style]
				border.Top.Color = &color
			case "bottom":
				border.Bottom.Style = styles[v.Style]
				border.Bottom.Color = &color
			case "diagonalUp":
				border.Diagonal.Style = styles[v.Style]
				border.Diagonal.Color = &color
				border.DiagonalUp = true
			case "diagonalDown":
				border.Diagonal.Style = styles[v.Style]
				border.Diagonal.Color = &color
				border.DiagonalDown = true
			}
		}
	}
	return &border
}

// setCellXfs provides function to set describes all of the formatting for a
// cell.
func setCellXfs(style *xlsxStyleSheet, fontID, numFmtID, fillID, borderID int, applyAlignment, applyProtection bool, alignment *xlsxAlignment, protection *xlsxProtection) int {
	var xf xlsxXf
	xf.FontID = fontID
	if fontID != 0 {
		xf.ApplyFont = true
	}
	xf.NumFmtID = numFmtID
	if numFmtID != 0 {
		xf.ApplyNumberFormat = true
	}
	xf.FillID = fillID
	xf.BorderID = borderID
	style.CellXfs.Count++
	xf.Alignment = alignment
	xf.ApplyAlignment = applyAlignment
	if applyProtection {
		xf.ApplyProtection = applyProtection
		xf.Protection = protection
	}
	xfID := 0
	xf.XfID = &xfID
	style.CellXfs.Xf = append(style.CellXfs.Xf, xf)
	return style.CellXfs.Count - 1
}

// SetCellStyle provides function to add style attribute for cells by given
// worksheet name, coordinate area and style ID. Note that diagonalDown and
// diagonalUp type border should be use same color in the same coordinate area.
//
// For example create a borders of cell H9 on Sheet1:
//
//    style, err := xlsx.NewStyle(`{"border":[{"type":"left","color":"0000FF","style":3},{"type":"top","color":"00FF00","style":4},{"type":"bottom","color":"FFFF00","style":5},{"type":"right","color":"FF0000","style":6},{"type":"diagonalDown","color":"A020F0","style":7},{"type":"diagonalUp","color":"A020F0","style":8}]}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Set gradient fill with vertical variants shading styles for cell H9 on
// Sheet1:
//
//    style, err := xlsx.NewStyle(`{"fill":{"type":"gradient","color":["#FFFFFF","#E0EBF5"],"shading":1}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Set solid style pattern fill for cell H9 on Sheet1:
//
//    style, err := xlsx.NewStyle(`{"fill":{"type":"pattern","color":["#E0EBF5"],"pattern":1}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Set alignment style for cell H9 on Sheet1:
//
//    style, err := xlsx.NewStyle(`{"alignment":{"horizontal":"center","ident":1,"justify_last_line":true,"reading_order":0,"relative_indent":1,"shrink_to_fit":true,"text_rotation":45,"vertical":"","wrap_text":true}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Dates and times in Excel are represented by real numbers, for example "Apr 7
// 2017 12:00 PM" is represented by the number 42920.5. Set date and time format
// for cell H9 on Sheet1:
//
//    xlsx.SetCellValue("Sheet1", "H9", 42920.5)
//    style, err := xlsx.NewStyle(`{"number_format": 22}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Set font style for cell H9 on Sheet1:
//
//    style, err := xlsx.NewStyle(`{"font":{"bold":true,"italic":true,"family":"Berlin Sans FB Demi","size":36,"color":"#777777"}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
// Hide and lock for cell H9 on Sheet1:
//
//    style, err := xlsx.NewStyle(`{"protection":{"hidden":true, "locked":true}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetCellStyle("Sheet1", "H9", "H9", style)
//
func (f *File) SetCellStyle(sheet, hcell, vcell string, styleID int) {
	hcell = strings.ToUpper(hcell)
	vcell = strings.ToUpper(vcell)

	// Coordinate conversion, convert C1:B3 to 2,0,1,2.
	hcol := string(strings.Map(letterOnlyMapF, hcell))
	hrow, err := strconv.Atoi(strings.Map(intOnlyMapF, hcell))
	if err != nil {
		return
	}
	hyAxis := hrow - 1
	hxAxis := TitleToNumber(hcol)

	vcol := string(strings.Map(letterOnlyMapF, vcell))
	vrow, err := strconv.Atoi(strings.Map(intOnlyMapF, vcell))
	if err != nil {
		return
	}
	vyAxis := vrow - 1
	vxAxis := TitleToNumber(vcol)

	// Correct the coordinate area, such correct C1:B3 to B1:C3.
	if vxAxis < hxAxis {
		vxAxis, hxAxis = hxAxis, vxAxis
	}

	if vyAxis < hyAxis {
		vyAxis, hyAxis = hyAxis, vyAxis
	}

	xlsx := f.workSheetReader(sheet)

	completeRow(xlsx, vyAxis+1, vxAxis+1)
	completeCol(xlsx, vyAxis+1, vxAxis+1)

	for r := hyAxis; r <= vyAxis; r++ {
		for k := hxAxis; k <= vxAxis; k++ {
			xlsx.SheetData.Row[r].C[k].S = styleID
		}
	}
}

// SetConditionalFormat provides function to create conditional formatting rule
// for cell value. Conditional formatting is a feature of Excel which allows you
// to apply a format to a cell or a range of cells based on certain criteria.
//
// The type option is a required parameter and it has no default value.
// Allowable type values and their associated parameters are:
//
//     Type          | Parameters
//    ---------------+------------------------------------
//     cell          | criteria
//                   | value
//                   | minimum
//                   | maximum
//     date          | criteria
//                   | value
//                   | minimum
//                   | maximum
//     time_period   | criteria
//     text          | criteria
//                   | value
//     average       | criteria
//     duplicate     | (none)
//     unique        | (none)
//     top           | criteria
//                   | value
//     bottom        | criteria
//                   | value
//     blanks        | (none)
//     no_blanks     | (none)
//     errors        | (none)
//     no_errors     | (none)
//     2_color_scale | min_type
//                   | max_type
//                   | min_value
//                   | max_value
//                   | min_color
//                   | max_color
//     3_color_scale | min_type
//                   | mid_type
//                   | max_type
//                   | min_value
//                   | mid_value
//                   | max_value
//                   | min_color
//                   | mid_color
//                   | max_color
//     data_bar      | min_type
//                   | max_type
//                   | min_value
//                   | max_value
//                   | bar_color
//     formula       | criteria
//
// The criteria parameter is used to set the criteria by which the cell data
// will be evaluated. It has no default value. The most common criteria as
// applied to {'type': 'cell'} are:
//
//    between                  |
//    not between              |
//    equal to                 | ==
//    not equal to             | !=
//    greater than             | >
//    less than                | <
//    greater than or equal to | >=
//    less than or equal to    | <=
//
// You can either use Excel's textual description strings, in the first column
// above, or the more common symbolic alternatives.
//
// Additional criteria which are specific to other conditional format types are
// shown in the relevant sections below.
//
// value: The value is generally used along with the criteria parameter to set
// the rule by which the cell data will be evaluated:
//
//    xlsx.SetConditionalFormat("Sheet1", "D1:D10", fmt.Sprintf(`[{"type":"cell","criteria":">","format":%d,"value":"6"}]`, format))
//
// The value property can also be an cell reference:
//
//    xlsx.SetConditionalFormat("Sheet1", "D1:D10", fmt.Sprintf(`[{"type":"cell","criteria":">","format":%d,"value":"$C$1"}]`, format))
//
// type: format - The format parameter is used to specify the format that will
// be applied to the cell when the conditional formatting criterion is met. The
// format is created using the NewConditionalStyle() method in the same way as
// cell formats:
//
//    format, err = xlsx.NewConditionalStyle(`{"font":{"color":"#9A0511"},"fill":{"type":"pattern","color":["#FEC7CE"],"pattern":1}}`)
//    if err != nil {
//        fmt.Println(err)
//    }
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"cell","criteria":">","format":%d,"value":"6"}]`, format))
//
// Note: In Excel, a conditional format is superimposed over the existing cell
// format and not all cell format properties can be modified. Properties that
// cannot be modified in a conditional format are font name, font size,
// superscript and subscript, diagonal borders, all alignment properties and all
// protection properties.
//
// Excel specifies some default formats to be used with conditional formatting.
// These can be replicated using the following excelize formats:
//
//    // Rose format for bad conditional.
//    format1, err = xlsx.NewConditionalStyle(`{"font":{"color":"#9A0511"},"fill":{"type":"pattern","color":["#FEC7CE"],"pattern":1}}`)
//
//    // Light yellow format for neutral conditional.
//    format2, err = xlsx.NewConditionalStyle(`{"font":{"color":"#9B5713"},"fill":{"type":"pattern","color":["#FEEAA0"],"pattern":1}}`)
//
//    // Light green format for good conditional.
//    format3, err = xlsx.NewConditionalStyle(`{"font":{"color":"#09600B"},"fill":{"type":"pattern","color":["#C7EECF"],"pattern":1}}`)
//
// type: minimum - The minimum parameter is used to set the lower limiting value
// when the criteria is either "between" or "not between".
//
//    // Hightlight cells rules: between...
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"cell","criteria":"between","format":%d,"minimum":"6","maximum":"8"}]`, format))
//
// type: maximum - The maximum parameter is used to set the upper limiting value
// when the criteria is either "between" or "not between". See the previous
// example.
//
// type: average - The average type is used to specify Excel's "Average" style
// conditional format:
//
//    // Top/Bottom rules: Above Average...
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"average","criteria":"=","format":%d, "above_average": true}]`, format1))
//
//    // Top/Bottom rules: Below Average...
//    xlsx.SetConditionalFormat("Sheet1", "B1:B10", fmt.Sprintf(`[{"type":"average","criteria":"=","format":%d, "above_average": false}]`, format2))
//
// type: duplicate - The duplicate type is used to highlight duplicate cells in a range:
//
//    // Hightlight cells rules: Duplicate Values...
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"duplicate","criteria":"=","format":%d}]`, format))
//
// type: unique - The unique type is used to highlight unique cells in a range:
//
//    // Hightlight cells rules: Not Equal To...
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"unique","criteria":"=","format":%d}]`, format))
//
// type: top - The top type is used to specify the top n values by number or percentage in a range:
//
//    // Top/Bottom rules: Top 10.
//    xlsx.SetConditionalFormat("Sheet1", "H1:H10", fmt.Sprintf(`[{"type":"top","criteria":"=","format":%d,"value":"6"}]`, format))
//
// The criteria can be used to indicate that a percentage condition is required:
//
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", fmt.Sprintf(`[{"type":"top","criteria":"=","format":%d,"value":"6","percent":true}]`, format))
//
// type: 2_color_scale - The 2_color_scale type is used to specify Excel's "2
// Color Scale" style conditional format:
//
//    // Color scales: 2 color.
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", `[{"type":"2_color_scale","criteria":"=","min_type":"min","max_type":"max","min_color":"#F8696B","max_color":"#63BE7B"}]`)
//
// This conditional type can be modified with min_type, max_type, min_value,
// max_value, min_color and max_color, see below.
//
// type: 3_color_scale - The 3_color_scale type is used to specify Excel's "3
// Color Scale" style conditional format:
//
//    // Color scales: 3 color.
//    xlsx.SetConditionalFormat("Sheet1", "A1:A10", `[{"type":"3_color_scale","criteria":"=","min_type":"min","mid_type":"percentile","max_type":"max","min_color":"#F8696B","mid_color":"#FFEB84","max_color":"#63BE7B"}]`)
//
// This conditional type can be modified with min_type, mid_type, max_type,
// min_value, mid_value, max_value, min_color, mid_color and max_color, see
// below.
//
// type: data_bar - The data_bar type is used to specify Excel's "Data Bar"
// style conditional format.
//
// min_type - The min_type and max_type properties are available when the conditional formatting type is 2_color_scale, 3_color_scale or data_bar. The mid_type is available for 3_color_scale. The properties are used as follows:
//
//    // Data Bars: Gradient Fill.
//    xlsx.SetConditionalFormat("Sheet1", "K1:K10", `[{"type":"data_bar", "criteria":"=", "min_type":"min","max_type":"max","bar_color":"#638EC6"}]`)
//
// The available min/mid/max types are:
//
//    min        (for min_type only)
//    num
//    percent
//    percentile
//    formula
//    max        (for max_type only)
//
// mid_type - Used for 3_color_scale. Same as min_type, see above.
//
// max_type - Same as min_type, see above.
//
// min_value - The min_value and max_value properties are available when the
// conditional formatting type is 2_color_scale, 3_color_scale or data_bar. The
// mid_value is available for 3_color_scale.
//
// mid_value - Used for 3_color_scale. Same as min_value, see above.
//
// max_value - Same as min_value, see above.
//
// min_color - The min_color and max_color properties are available when the
// conditional formatting type is 2_color_scale, 3_color_scale or data_bar.
// The mid_color is available for 3_color_scale. The properties are used as
// follows:
//
//    // Color scales: 3 color.
//    xlsx.SetConditionalFormat("Sheet1", "B1:B10", `[{"type":"3_color_scale","criteria":"=","min_type":"min","mid_type":"percentile","max_type":"max","min_color":"#F8696B","mid_color":"#FFEB84","max_color":"#63BE7B"}]`)
//
// mid_color - Used for 3_color_scale. Same as min_color, see above.
//
// max_color - Same as min_color, see above.
//
// bar_color - Used for data_bar. Same as min_color, see above.
//
func (f *File) SetConditionalFormat(sheet, area, formatSet string) error {
	var format []*formatConditional
	err := json.Unmarshal([]byte(formatSet), &format)
	if err != nil {
		return err
	}
	drawContFmtFunc := map[string]func(p int, ct string, fmtCond *formatConditional) *xlsxCfRule{
		"cellIs":          drawCondFmtCellIs,
		"top10":           drawCondFmtTop10,
		"aboveAverage":    drawCondFmtAboveAverage,
		"duplicateValues": drawCondFmtDuplicateUniqueValues,
		"uniqueValues":    drawCondFmtDuplicateUniqueValues,
		"2_color_scale":   drawCondFmtColorScale,
		"3_color_scale":   drawCondFmtColorScale,
		"dataBar":         drawCondFmtDataBar,
		"expression":      drawConfFmtExp,
	}

	xlsx := f.workSheetReader(sheet)
	cfRule := []*xlsxCfRule{}
	for p, v := range format {
		var vt, ct string
		var ok bool
		// "type" is a required parameter, check for valid validation types.
		vt, ok = validType[v.Type]
		if ok {
			// Check for valid criteria types.
			ct, ok = criteriaType[v.Criteria]
			if ok || vt == "expression" {
				drawfunc, ok := drawContFmtFunc[vt]
				if ok {
					cfRule = append(cfRule, drawfunc(p, ct, v))
				}
			}
		}
	}

	xlsx.ConditionalFormatting = append(xlsx.ConditionalFormatting, &xlsxConditionalFormatting{
		SQRef:  area,
		CfRule: cfRule,
	})
	return err
}

// drawCondFmtCellIs provides function to create conditional formatting rule for
// cell value (include between, not between, equal, not equal, greater than and
// less than) by given priority, criteria type and format settings.
func drawCondFmtCellIs(p int, ct string, format *formatConditional) *xlsxCfRule {
	c := &xlsxCfRule{
		Priority: p + 1,
		Type:     validType[format.Type],
		Operator: ct,
		DxfID:    &format.Format,
	}
	// "between" and "not between" criteria require 2 values.
	_, ok := map[string]bool{"between": true, "notBetween": true}[ct]
	if ok {
		c.Formula = append(c.Formula, format.Minimum)
		c.Formula = append(c.Formula, format.Maximum)
	}
	_, ok = map[string]bool{"equal": true, "notEqual": true, "greaterThan": true, "lessThan": true}[ct]
	if ok {
		c.Formula = append(c.Formula, format.Value)
	}
	return c
}

// drawCondFmtTop10 provides function to create conditional formatting rule for
// top N (default is top 10) by given priority, criteria type and format
// settings.
func drawCondFmtTop10(p int, ct string, format *formatConditional) *xlsxCfRule {
	c := &xlsxCfRule{
		Priority: p + 1,
		Type:     validType[format.Type],
		Rank:     10,
		DxfID:    &format.Format,
		Percent:  format.Percent,
	}
	rank, err := strconv.Atoi(format.Value)
	if err == nil {
		c.Rank = rank
	}
	return c
}

// drawCondFmtAboveAverage provides function to create conditional formatting
// rule for above average and below average by given priority, criteria type and
// format settings.
func drawCondFmtAboveAverage(p int, ct string, format *formatConditional) *xlsxCfRule {
	return &xlsxCfRule{
		Priority:     p + 1,
		Type:         validType[format.Type],
		AboveAverage: &format.AboveAverage,
		DxfID:        &format.Format,
	}
}

// drawCondFmtDuplicateUniqueValues provides function to create conditional
// formatting rule for duplicate and unique values by given priority, criteria
// type and format settings.
func drawCondFmtDuplicateUniqueValues(p int, ct string, format *formatConditional) *xlsxCfRule {
	return &xlsxCfRule{
		Priority: p + 1,
		Type:     validType[format.Type],
		DxfID:    &format.Format,
	}
}

// drawCondFmtColorScale provides function to create conditional formatting rule
// for color scale (include 2 color scale and 3 color scale) by given priority,
// criteria type and format settings.
func drawCondFmtColorScale(p int, ct string, format *formatConditional) *xlsxCfRule {
	c := &xlsxCfRule{
		Priority: p + 1,
		Type:     "colorScale",
		ColorScale: &xlsxColorScale{
			Cfvo: []*xlsxCfvo{
				{Type: format.MinType},
			},
			Color: []*xlsxColor{
				{RGB: getPaletteColor(format.MinColor)},
			},
		},
	}
	if validType[format.Type] == "3_color_scale" {
		c.ColorScale.Cfvo = append(c.ColorScale.Cfvo, &xlsxCfvo{Type: format.MidType, Val: 50})
		c.ColorScale.Color = append(c.ColorScale.Color, &xlsxColor{RGB: getPaletteColor(format.MidColor)})
	}
	c.ColorScale.Cfvo = append(c.ColorScale.Cfvo, &xlsxCfvo{Type: format.MaxType})
	c.ColorScale.Color = append(c.ColorScale.Color, &xlsxColor{RGB: getPaletteColor(format.MaxColor)})
	return c
}

// drawCondFmtDataBar provides function to create conditional formatting rule
// for data bar by given priority, criteria type and format settings.
func drawCondFmtDataBar(p int, ct string, format *formatConditional) *xlsxCfRule {
	return &xlsxCfRule{
		Priority: p + 1,
		Type:     validType[format.Type],
		DataBar: &xlsxDataBar{
			Cfvo:  []*xlsxCfvo{{Type: format.MinType}, {Type: format.MaxType}},
			Color: []*xlsxColor{{RGB: getPaletteColor(format.BarColor)}},
		},
	}
}

// drawConfFmtExp provides function to create conditional formatting rule for
// expression by given priority, criteria type and format settings.
func drawConfFmtExp(p int, ct string, format *formatConditional) *xlsxCfRule {
	return &xlsxCfRule{
		Priority: p + 1,
		Type:     validType[format.Type],
		Formula:  []string{format.Criteria},
		DxfID:    &format.Format,
	}
}

// getPaletteColor provides function to convert the RBG color by given string.
func getPaletteColor(color string) string {
	return "FF" + strings.Replace(strings.ToUpper(color), "#", "", -1)
}
