// Package chardet ports character set detection from ICU.
package chardet

import (
	"errors"
	"sort"
)

// Result contains all the information that charset detector gives.
type Result struct {
	// IANA name of the detected charset.
	Charset string
	// IANA name of the detected language. It may be empty for some charsets.
	Language string
	// Confidence of the Result. Scale from 1 to 100. The bigger, the more confident.
	Confidence int
}

// Detector implements charset detection.
type Detector struct {
	recognizers []recognizer
	stripTag    bool
}

// List of charset recognizers
var recognizers = []recognizer{
	newRecognizer_utf8(),
	newRecognizer_utf16be(),
	newRecognizer_utf16le(),
	newRecognizer_utf32be(),
	newRecognizer_utf32le(),
	newRecognizer_8859_1_en(),
	newRecognizer_8859_1_da(),
	newRecognizer_8859_1_de(),
	newRecognizer_8859_1_es(),
	newRecognizer_8859_1_fr(),
	newRecognizer_8859_1_it(),
	newRecognizer_8859_1_nl(),
	newRecognizer_8859_1_no(),
	newRecognizer_8859_1_pt(),
	newRecognizer_8859_1_sv(),
	newRecognizer_8859_2_cs(),
	newRecognizer_8859_2_hu(),
	newRecognizer_8859_2_pl(),
	newRecognizer_8859_2_ro(),
	newRecognizer_8859_5_ru(),
	newRecognizer_8859_6_ar(),
	newRecognizer_8859_7_el(),
	newRecognizer_8859_8_I_he(),
	newRecognizer_8859_8_he(),
	newRecognizer_windows_1251(),
	newRecognizer_windows_1256(),
	newRecognizer_KOI8_R(),
	newRecognizer_8859_9_tr(),

	newRecognizer_sjis(),
	newRecognizer_gb_18030(),
	newRecognizer_euc_jp(),
	newRecognizer_euc_kr(),
	newRecognizer_big5(),

	newRecognizer_2022JP(),
	newRecognizer_2022KR(),
	newRecognizer_2022CN(),

	newRecognizer_IBM424_he_rtl(),
	newRecognizer_IBM424_he_ltr(),
	newRecognizer_IBM420_ar_rtl(),
	newRecognizer_IBM420_ar_ltr(),
}

// NewTextDetector creates a Detector for plain text.
func NewTextDetector() *Detector {
	return &Detector{recognizers, false}
}

// NewHtmlDetector creates a Detector for Html.
func NewHtmlDetector() *Detector {
	return &Detector{recognizers, true}
}

var (
	NotDetectedError = errors.New("Charset not detected.")
)

// DetectBest returns the Result with highest Confidence.
func (d *Detector) DetectBest(b []byte) (r *Result, err error) {
	var all []Result
	if all, err = d.DetectAll(b); err == nil {
		r = &all[0]
	}
	return
}

// DetectAll returns all Results which have non-zero Confidence. The Results are sorted by Confidence in descending order.
func (d *Detector) DetectAll(b []byte) ([]Result, error) {
	input := newRecognizerInput(b, d.stripTag)
	outputChan := make(chan recognizerOutput)
	for _, r := range d.recognizers {
		go matchHelper(r, input, outputChan)
	}
	outputs := make([]recognizerOutput, 0, len(d.recognizers))
	for i := 0; i < len(d.recognizers); i++ {
		o := <-outputChan
		if o.Confidence > 0 {
			outputs = append(outputs, o)
		}
	}
	if len(outputs) == 0 {
		return nil, NotDetectedError
	}

	sort.Sort(recognizerOutputs(outputs))
	dedupOutputs := make([]Result, 0, len(outputs))
	foundCharsets := make(map[string]struct{}, len(outputs))
	for _, o := range outputs {
		if _, found := foundCharsets[o.Charset]; !found {
			dedupOutputs = append(dedupOutputs, Result(o))
			foundCharsets[o.Charset] = struct{}{}
		}
	}
	if len(dedupOutputs) == 0 {
		return nil, NotDetectedError
	}
	return dedupOutputs, nil
}

func matchHelper(r recognizer, input *recognizerInput, outputChan chan<- recognizerOutput) {
	outputChan <- r.Match(input)
}

type recognizerOutputs []recognizerOutput

func (r recognizerOutputs) Len() int           { return len(r) }
func (r recognizerOutputs) Less(i, j int) bool { return r[i].Confidence > r[j].Confidence }
func (r recognizerOutputs) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
