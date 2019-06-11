package validator

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
)

type tagType uint8

const (
	typeDefault tagType = iota
	typeOmitEmpty
	typeNoStructLevel
	typeStructOnly
	typeDive
	typeOr
	typeExists
)

type structCache struct {
	lock sync.Mutex
	m    atomic.Value // map[reflect.Type]*cStruct
}

func (sc *structCache) Get(key reflect.Type) (c *cStruct, found bool) {
	c, found = sc.m.Load().(map[reflect.Type]*cStruct)[key]
	return
}

func (sc *structCache) Set(key reflect.Type, value *cStruct) {

	m := sc.m.Load().(map[reflect.Type]*cStruct)

	nm := make(map[reflect.Type]*cStruct, len(m)+1)
	for k, v := range m {
		nm[k] = v
	}
	nm[key] = value
	sc.m.Store(nm)
}

type tagCache struct {
	lock sync.Mutex
	m    atomic.Value // map[string]*cTag
}

func (tc *tagCache) Get(key string) (c *cTag, found bool) {
	c, found = tc.m.Load().(map[string]*cTag)[key]
	return
}

func (tc *tagCache) Set(key string, value *cTag) {

	m := tc.m.Load().(map[string]*cTag)

	nm := make(map[string]*cTag, len(m)+1)
	for k, v := range m {
		nm[k] = v
	}
	nm[key] = value
	tc.m.Store(nm)
}

type cStruct struct {
	Name   string
	fields map[int]*cField
	fn     StructLevelFunc
}

type cField struct {
	Idx     int
	Name    string
	AltName string
	cTags   *cTag
}

type cTag struct {
	tag            string
	aliasTag       string
	actualAliasTag string
	param          string
	hasAlias       bool
	typeof         tagType
	hasTag         bool
	fn             Func
	next           *cTag
}

func (v *Validate) extractStructCache(current reflect.Value, sName string) *cStruct {

	v.structCache.lock.Lock()
	defer v.structCache.lock.Unlock() // leave as defer! because if inner panics, it will never get unlocked otherwise!

	typ := current.Type()

	// could have been multiple trying to access, but once first is done this ensures struct
	// isn't parsed again.
	cs, ok := v.structCache.Get(typ)
	if ok {
		return cs
	}

	cs = &cStruct{Name: sName, fields: make(map[int]*cField), fn: v.structLevelFuncs[typ]}

	numFields := current.NumField()

	var ctag *cTag
	var fld reflect.StructField
	var tag string
	var customName string

	for i := 0; i < numFields; i++ {

		fld = typ.Field(i)

		if !fld.Anonymous && fld.PkgPath != blank {
			continue
		}

		tag = fld.Tag.Get(v.tagName)

		if tag == skipValidationTag {
			continue
		}

		customName = fld.Name

		if v.fieldNameTag != blank {

			name := strings.SplitN(fld.Tag.Get(v.fieldNameTag), ",", 2)[0]

			// dash check is for json "-" (aka skipValidationTag) means don't output in json
			if name != "" && name != skipValidationTag {
				customName = name
			}
		}

		// NOTE: cannot use shared tag cache, because tags may be equal, but things like alias may be different
		// and so only struct level caching can be used instead of combined with Field tag caching

		if len(tag) > 0 {
			ctag, _ = v.parseFieldTagsRecursive(tag, fld.Name, blank, false)
		} else {
			// even if field doesn't have validations need cTag for traversing to potential inner/nested
			// elements of the field.
			ctag = new(cTag)
		}

		cs.fields[i] = &cField{Idx: i, Name: fld.Name, AltName: customName, cTags: ctag}
	}

	v.structCache.Set(typ, cs)

	return cs
}

func (v *Validate) parseFieldTagsRecursive(tag string, fieldName string, alias string, hasAlias bool) (firstCtag *cTag, current *cTag) {

	var t string
	var ok bool
	noAlias := len(alias) == 0
	tags := strings.Split(tag, tagSeparator)

	for i := 0; i < len(tags); i++ {

		t = tags[i]

		if noAlias {
			alias = t
		}

		if v.hasAliasValidators {
			// check map for alias and process new tags, otherwise process as usual
			if tagsVal, found := v.aliasValidators[t]; found {

				if i == 0 {
					firstCtag, current = v.parseFieldTagsRecursive(tagsVal, fieldName, t, true)
				} else {
					next, curr := v.parseFieldTagsRecursive(tagsVal, fieldName, t, true)
					current.next, current = next, curr

				}

				continue
			}
		}

		if i == 0 {
			current = &cTag{aliasTag: alias, hasAlias: hasAlias, hasTag: true}
			firstCtag = current
		} else {
			current.next = &cTag{aliasTag: alias, hasAlias: hasAlias, hasTag: true}
			current = current.next
		}

		switch t {

		case diveTag:
			current.typeof = typeDive
			continue

		case omitempty:
			current.typeof = typeOmitEmpty
			continue

		case structOnlyTag:
			current.typeof = typeStructOnly
			continue

		case noStructLevelTag:
			current.typeof = typeNoStructLevel
			continue

		case existsTag:
			current.typeof = typeExists
			continue

		default:

			// if a pipe character is needed within the param you must use the utf8Pipe representation "0x7C"
			orVals := strings.Split(t, orSeparator)

			for j := 0; j < len(orVals); j++ {

				vals := strings.SplitN(orVals[j], tagKeySeparator, 2)

				if noAlias {
					alias = vals[0]
					current.aliasTag = alias
				} else {
					current.actualAliasTag = t
				}

				if j > 0 {
					current.next = &cTag{aliasTag: alias, actualAliasTag: current.actualAliasTag, hasAlias: hasAlias, hasTag: true}
					current = current.next
				}

				current.tag = vals[0]
				if len(current.tag) == 0 {
					panic(strings.TrimSpace(fmt.Sprintf(invalidValidation, fieldName)))
				}

				if current.fn, ok = v.validationFuncs[current.tag]; !ok {
					panic(strings.TrimSpace(fmt.Sprintf(undefinedValidation, fieldName)))
				}

				if len(orVals) > 1 {
					current.typeof = typeOr
				}

				if len(vals) > 1 {
					current.param = strings.Replace(strings.Replace(vals[1], utf8HexComma, ",", -1), utf8Pipe, "|", -1)
				}
			}
		}
	}

	return
}
