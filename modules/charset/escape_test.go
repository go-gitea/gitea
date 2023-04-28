// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"reflect"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/translation"
)

type escapeControlTest struct {
	name   string
	text   string
	status EscapeStatus
	result string
}

var escapeControlTests = []escapeControlTest{
	{
		name: "<empty>",
	},
	{
		name:   "single line western",
		text:   "single line western",
		result: "single line western",
		status: EscapeStatus{},
	},
	{
		name:   "multi line western",
		text:   "single line western\nmulti line western\n",
		result: "single line western\nmulti line western\n",
		status: EscapeStatus{},
	},
	{
		name:   "multi line western non-breaking space",
		text:   "single line western\nmulti line western\n",
		result: `single line<span class="escaped-code-point" data-escaped="[U+00A0]"><span class="char"> </span></span>western` + "\n" + `multi line<span class="escaped-code-point" data-escaped="[U+00A0]"><span class="char"> </span></span>western` + "\n",
		status: EscapeStatus{Escaped: true, HasInvisible: true},
	},
	{
		name:   "mixed scripts: western + japanese",
		text:   "日属秘ぞしちゅ。Then some western.",
		result: "日属秘ぞしちゅ。Then some western.",
		status: EscapeStatus{},
	},
	{
		name:   "japanese",
		text:   "日属秘ぞしちゅ。",
		result: "日属秘ぞしちゅ。",
		status: EscapeStatus{},
	},
	{
		name:   "hebrew",
		text:   "עד תקופת יוון העתיקה היה העיסוק במתמטיקה תכליתי בלבד: היא שימשה כאוסף של נוסחאות לחישוב קרקע, אוכלוסין וכו'. פריצת הדרך של היוונים, פרט לתרומותיהם הגדולות לידע המתמטי, הייתה בלימוד המתמטיקה כשלעצמה, מתוקף ערכה הרוחני. יחסם של חלק מהיוונים הקדמונים למתמטיקה היה דתי - למשל, הכת שאסף סביבו פיתגורס האמינה כי המתמטיקה היא הבסיס לכל הדברים. היוונים נחשבים ליוצרי מושג ההוכחה המתמטית, וכן לראשונים שעסקו במתמטיקה לשם עצמה, כלומר כתחום מחקרי עיוני ומופשט ולא רק כעזר שימושי. עם זאת, לצדה",
		result: `עד תקופת <span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">י</span></span><span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ו</span></span><span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ו</span></span><span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ן</span></span> העתיקה היה העיסוק במתמטיקה תכליתי בלבד: היא שימשה כאוסף של נוסחאות לחישוב קרקע, אוכלוסין וכו&#39;. פריצת הדרך של היוונים, פרט לתרומותיהם הגדולות לידע המתמטי, הייתה בלימוד המתמטיקה כשלעצמה, מתוקף ערכה הרוחני. יחסם של חלק מהיוונים הקדמונים למתמטיקה היה דתי - למשל, הכת שאסף סביבו פיתגורס האמינה כי המתמטיקה היא הבסיס לכל הדברים. היוונים נחשבים ליוצרי מושג ההוכחה המתמטית, וכן לראשונים שעסקו במתמטיקה לשם עצמה, כלומר כתחום מחקרי עיוני ומופשט ולא רק כעזר שימושי. עם זאת, לצדה`,
		status: EscapeStatus{Escaped: true, HasAmbiguous: true},
	},
	{
		name: "more hebrew",
		text: `בתקופה מאוחרת יותר, השתמשו היוונים בשיטת סימון מתקדמת יותר, שבה הוצגו המספרים לפי 22 אותיות האלפבית היווני. לסימון המספרים בין 1 ל-9 נקבעו תשע האותיות הראשונות, בתוספת גרש ( ' ) בצד ימין של האות, למעלה; תשע האותיות הבאות ייצגו את העשרות מ-10 עד 90, והבאות את המאות. לסימון הספרות בין 1000 ל-900,000, השתמשו היוונים באותן אותיות, אך הוסיפו לאותיות את הגרש דווקא מצד שמאל של האותיות, למטה. ממיליון ומעלה, כנראה השתמשו היוונים בשני תגים במקום אחד.

			המתמטיקאי הבולט הראשון ביוון העתיקה, ויש האומרים בתולדות האנושות, הוא תאלס (624 לפנה"ס - 546 לפנה"ס בקירוב).[1] לא יהיה זה משולל יסוד להניח שהוא האדם הראשון שהוכיח משפט מתמטי, ולא רק גילה אותו. תאלס הוכיח שישרים מקבילים חותכים מצד אחד של שוקי זווית קטעים בעלי יחסים שווים (משפט תאלס הראשון), שהזווית המונחת על קוטר במעגל היא זווית ישרה (משפט תאלס השני), שהקוטר מחלק את המעגל לשני חלקים שווים, ושזוויות הבסיס במשולש שווה-שוקיים שוות זו לזו. מיוחסות לו גם שיטות למדידת גובהן של הפירמידות בעזרת מדידת צילן ולקביעת מיקומה של ספינה הנראית מן החוף.

			בשנים 582 לפנה"ס עד 496 לפנה"ס, בקירוב, חי מתמטיקאי חשוב במיוחד - פיתגורס. המקורות הראשוניים עליו מועטים, וההיסטוריונים מתקשים להפריד את העובדות משכבת המסתורין והאגדות שנקשרו בו. ידוע שסביבו התקבצה האסכולה הפיתגוראית מעין כת פסבדו-מתמטית שהאמינה ש"הכל מספר", או ליתר דיוק הכל ניתן לכימות, וייחסה למספרים משמעויות מיסטיות. ככל הנראה הפיתגוראים ידעו לבנות את הגופים האפלטוניים, הכירו את הממוצע האריתמטי, הממוצע הגאומטרי והממוצע ההרמוני והגיעו להישגים חשובים נוספים. ניתן לומר שהפיתגוראים גילו את היותו של השורש הריבועי של 2, שהוא גם האלכסון בריבוע שאורך צלעותיו 1, אי רציונלי, אך תגליתם הייתה למעשה רק שהקטעים "חסרי מידה משותפת", ומושג המספר האי רציונלי מאוחר יותר.[2] אזכור ראשון לקיומם של קטעים חסרי מידה משותפת מופיע בדיאלוג "תאיטיטוס" של אפלטון, אך רעיון זה היה מוכר עוד קודם לכן, במאה החמישית לפנה"ס להיפאסוס, בן האסכולה הפיתגוראית, ואולי לפיתגורס עצמו.[3]`,
		result: `בתקופה מאוחרת יותר, השתמשו היוונים בשיטת סימון מתקדמת יותר, שבה הוצגו המספרים לפי 22 אותיות האלפבית היווני. לסימון המספרים בין 1 ל-9 נקבעו תשע האותיות הראשונות, בתוספת גרש ( &#39; ) בצד ימין של האות, למעלה; תשע האותיות הבאות ייצגו את העשרות מ-10 עד 90, והבאות את המאות. לסימון הספרות בין 1000 ל-900,000, השתמשו היוונים באותן אותיות, אך הוסיפו לאותיות את הגרש דווקא מצד שמאל של האותיות, למטה. ממיליון ומעלה, כנראה השתמשו היוונים בשני תגים במקום אחד.

			המתמטיקאי הבולט הראשון ביוון העתיקה, ויש האומרים בתולדות האנושות, הוא תאלס (624 לפנה&#34;<span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ס</span></span> - 546 לפנה&#34;<span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ס</span></span> בקירוב).[1] לא יהיה זה משולל יסוד להניח שהוא האדם הראשון שהוכיח משפט מתמטי, ולא רק גילה אותו. תאלס הוכיח שישרים מקבילים חותכים מצד אחד של שוקי זווית קטעים בעלי יחסים שווים (משפט תאלס הראשון), שהזווית המונחת על קוטר במעגל היא זווית ישרה (משפט תאלס השני), שהקוטר מחלק את המעגל לשני חלקים שווים, ושזוויות הבסיס במשולש שווה-שוקיים שוות זו לזו. מיוחסות לו גם שיטות למדידת גובהן של הפירמידות בעזרת מדידת צילן ולקביעת מיקומה של ספינה הנראית מן החוף.

			בשנים 582 לפנה&#34;<span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ס</span></span> עד 496 לפנה&#34;<span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ס</span></span>, בקירוב, חי מתמטיקאי חשוב במיוחד - פיתגורס. המקורות הראשוניים עליו מועטים, וההיסטוריונים מתקשים להפריד את העובדות משכבת המסתורין והאגדות שנקשרו בו. ידוע שסביבו התקבצה האסכולה הפיתגוראית מעין כת פסבדו-מתמטית שהאמינה ש&#34;הכל מספר&#34;, או ליתר דיוק הכל ניתן לכימות, וייחסה למספרים משמעויות מיסטיות. ככל הנראה הפיתגוראים ידעו לבנות את הגופים האפלטוניים, הכירו את הממוצע האריתמטי, הממוצע הגאומטרי והממוצע ההרמוני והגיעו להישגים חשובים נוספים. ניתן לומר שהפיתגוראים גילו את היותו של השורש הריבועי של 2, שהוא גם האלכסון בריבוע שאורך צלעותיו 1, אי רציונלי, אך תגליתם הייתה למעשה רק שהקטעים &#34;חסרי מידה משותפת&#34;, ומושג המספר האי רציונלי מאוחר יותר.[2] אזכור ראשון לקיומם של קטעים חסרי מידה משותפת מופיע בדיאלוג &#34;תאיטיטוס&#34; של אפלטון, אך רעיון זה היה מוכר עוד קודם לכן, במאה החמישית לפנה&#34;<span class="ambiguous-code-point" data-tooltip-content="repo.ambiguous_character"><span class="char">ס</span></span> להיפאסוס, בן האסכולה הפיתגוראית, ואולי לפיתגורס עצמו.[3]`,
		status: EscapeStatus{Escaped: true, HasAmbiguous: true},
	},
	{
		name: "Mixed RTL+LTR",
		text: `Many computer programs fail to display bidirectional text correctly.
For example, the Hebrew name Sarah (שרה) is spelled: sin (ש) (which appears rightmost),
then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		result: `Many computer programs fail to display bidirectional text correctly.
For example, the Hebrew name Sarah (שרה) is spelled: sin (ש) (which appears rightmost),
then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		status: EscapeStatus{},
	},
	{
		name: "Mixed RTL+LTR+BIDI",
		text: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		result: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).`,
		status: EscapeStatus{},
	},
	{
		name:   "Accented characters",
		text:   string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}),
		result: string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba}),
		status: EscapeStatus{},
	},
	{
		name:   "Program",
		text:   "string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})",
		result: "string([]byte{0xc3, 0xa1, 0xc3, 0xa9, 0xc3, 0xad, 0xc3, 0xb3, 0xc3, 0xba})",
		status: EscapeStatus{},
	},
	{
		name:   "CVE testcase",
		text:   "if access_level != \"user\u202E \u2066// Check if admin\u2069 \u2066\" {",
		result: `if access_level != &#34;user<span class="escaped-code-point" data-escaped="[U+202E]"><span class="char">` + "\u202e" + `</span></span> <span class="escaped-code-point" data-escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>// Check if admin<span class="escaped-code-point" data-escaped="[U+2069]"><span class="char">` + "\u2069" + `</span></span> <span class="escaped-code-point" data-escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>&#34; {`,
		status: EscapeStatus{Escaped: true, HasInvisible: true},
	},
	{
		name: "Mixed testcase with fail",
		text: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).` +
			"\nif access_level != \"user\u202E \u2066// Check if admin\u2069 \u2066\" {\n",
		result: `Many computer programs fail to display bidirectional text correctly.
			For example, the Hebrew name Sarah ` + "\u2067" + `שרה` + "\u2066\n" +
			`sin (ש) (which appears rightmost), then resh (ר), and finally heh (ה) (which should appear leftmost).` +
			"\n" + `if access_level != &#34;user<span class="escaped-code-point" data-escaped="[U+202E]"><span class="char">` + "\u202e" + `</span></span> <span class="escaped-code-point" data-escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>// Check if admin<span class="escaped-code-point" data-escaped="[U+2069]"><span class="char">` + "\u2069" + `</span></span> <span class="escaped-code-point" data-escaped="[U+2066]"><span class="char">` + "\u2066" + `</span></span>&#34; {` + "\n",
		status: EscapeStatus{Escaped: true, HasInvisible: true},
	},
	{
		// UTF-8/16/32 all use the same codepoint for BOM
		// Gitea could read UTF-16/32 content and convert into UTF-8 internally then render it, so we only process UTF-8 internally
		name:   "UTF BOM",
		text:   "\xef\xbb\xbftest",
		result: "\xef\xbb\xbftest",
		status: EscapeStatus{},
	},
}

func TestEscapeControlString(t *testing.T) {
	for _, tt := range escapeControlTests {
		t.Run(tt.name, func(t *testing.T) {
			status, result := EscapeControlString(tt.text, &translation.MockLocale{})
			if !reflect.DeepEqual(*status, tt.status) {
				t.Errorf("EscapeControlString() status = %v, wanted= %v", status, tt.status)
			}
			if result != tt.result {
				t.Errorf("EscapeControlString()\nresult= %v,\nwanted= %v", result, tt.result)
			}
		})
	}
}

func TestEscapeControlReader(t *testing.T) {
	// lets add some control characters to the tests
	tests := make([]escapeControlTest, 0, len(escapeControlTests)*3)
	copy(tests, escapeControlTests)

	// if there is a BOM, we should keep the BOM
	addPrefix := func(prefix, s string) string {
		if strings.HasPrefix(s, "\xef\xbb\xbf") {
			return s[:3] + prefix + s[3:]
		}
		return prefix + s
	}
	for _, test := range escapeControlTests {
		test.name += " (+Control)"
		test.text = addPrefix("\u001E", test.text)
		test.result = addPrefix(`<span class="escaped-code-point" data-escaped="[U+001E]"><span class="char">`+"\u001e"+`</span></span>`, test.result)
		test.status.Escaped = true
		test.status.HasInvisible = true
		tests = append(tests, test)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.NewReader(tt.text)
			output := &strings.Builder{}
			status, err := EscapeControlReader(input, output, &translation.MockLocale{})
			result := output.String()
			if err != nil {
				t.Errorf("EscapeControlReader(): err = %v", err)
			}

			if !reflect.DeepEqual(*status, tt.status) {
				t.Errorf("EscapeControlReader() status = %v, wanted= %v", status, tt.status)
			}
			if result != tt.result {
				t.Errorf("EscapeControlReader()\nresult= %v,\nwanted= %v", result, tt.result)
			}
		})
	}
}

func TestEscapeControlReader_panic(t *testing.T) {
	bs := make([]byte, 0, 20479)
	bs = append(bs, 'A')
	for i := 0; i < 6826; i++ {
		bs = append(bs, []byte("—")...)
	}
	_, _ = EscapeControlString(string(bs), &translation.MockLocale{})
}
