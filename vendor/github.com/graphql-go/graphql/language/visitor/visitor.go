package visitor

import (
	"encoding/json"
	"reflect"

	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/typeInfo"
)

const (
	ActionNoChange = ""
	ActionBreak    = "BREAK"
	ActionSkip     = "SKIP"
	ActionUpdate   = "UPDATE"
)

type KeyMap map[string][]string

// note that the keys are in Capital letters, equivalent to the ast.Node field Names
var QueryDocumentKeys = KeyMap{
	"Name":     []string{},
	"Document": []string{"Definitions"},
	"OperationDefinition": []string{
		"Name",
		"VariableDefinitions",
		"Directives",
		"SelectionSet",
	},
	"VariableDefinition": []string{
		"Variable",
		"Type",
		"DefaultValue",
	},
	"Variable":     []string{"Name"},
	"SelectionSet": []string{"Selections"},
	"Field": []string{
		"Alias",
		"Name",
		"Arguments",
		"Directives",
		"SelectionSet",
	},
	"Argument": []string{
		"Name",
		"Value",
	},

	"FragmentSpread": []string{
		"Name",
		"Directives",
	},
	"InlineFragment": []string{
		"TypeCondition",
		"Directives",
		"SelectionSet",
	},
	"FragmentDefinition": []string{
		"Name",
		"TypeCondition",
		"Directives",
		"SelectionSet",
	},

	"IntValue":     []string{},
	"FloatValue":   []string{},
	"StringValue":  []string{},
	"BooleanValue": []string{},
	"EnumValue":    []string{},
	"ListValue":    []string{"Values"},
	"ObjectValue":  []string{"Fields"},
	"ObjectField": []string{
		"Name",
		"Value",
	},

	"Directive": []string{
		"Name",
		"Arguments",
	},

	"Named":   []string{"Name"},
	"List":    []string{"Type"},
	"NonNull": []string{"Type"},

	"SchemaDefinition": []string{
		"Directives",
		"OperationTypes",
	},
	"OperationTypeDefinition": []string{"Type"},

	"ScalarDefinition": []string{
		"Name",
		"Directives",
	},
	"ObjectDefinition": []string{
		"Name",
		"Interfaces",
		"Directives",
		"Fields",
	},
	"FieldDefinition": []string{
		"Name",
		"Arguments",
		"Type",
		"Directives",
	},
	"InputValueDefinition": []string{
		"Name",
		"Type",
		"DefaultValue",
		"Directives",
	},
	"InterfaceDefinition": []string{
		"Name",
		"Directives",
		"Fields",
	},
	"UnionDefinition": []string{
		"Name",
		"Directives",
		"Types",
	},
	"EnumDefinition": []string{
		"Name",
		"Directives",
		"Values",
	},
	"EnumValueDefinition": []string{
		"Name",
		"Directives",
	},
	"InputObjectDefinition": []string{
		"Name",
		"Directives",
		"Fields",
	},

	"TypeExtensionDefinition": []string{"Definition"},

	"DirectiveDefinition": []string{"Name", "Arguments", "Locations"},
}

type stack struct {
	Index   int
	Keys    []interface{}
	Edits   []*edit
	inSlice bool
	Prev    *stack
}
type edit struct {
	Key   interface{}
	Value interface{}
}

type VisitFuncParams struct {
	Node      interface{}
	Key       interface{}
	Parent    ast.Node
	Path      []interface{}
	Ancestors []ast.Node
}

type VisitFunc func(p VisitFuncParams) (string, interface{})

type NamedVisitFuncs struct {
	Kind  VisitFunc // 1) Named visitors triggered when entering a node a specific kind.
	Leave VisitFunc // 2) Named visitors that trigger upon entering and leaving a node of
	Enter VisitFunc // 2) Named visitors that trigger upon entering and leaving a node of
}

type VisitorOptions struct {
	KindFuncMap map[string]NamedVisitFuncs
	Enter       VisitFunc // 3) Generic visitors that trigger upon entering and leaving any node.
	Leave       VisitFunc // 3) Generic visitors that trigger upon entering and leaving any node.

	EnterKindMap map[string]VisitFunc // 4) Parallel visitors for entering and leaving nodes of a specific kind
	LeaveKindMap map[string]VisitFunc // 4) Parallel visitors for entering and leaving nodes of a specific kind
}

func Visit(root ast.Node, visitorOpts *VisitorOptions, keyMap KeyMap) interface{} {
	visitorKeys := keyMap
	if visitorKeys == nil {
		visitorKeys = QueryDocumentKeys
	}

	var (
		result         interface{}
		newRoot        ast.Node = root
		sstack         *stack
		parent         interface{}
		parentSlice    []interface{}
		inSlice        = false
		prevInSlice    = false
		keys           = []interface{}{root}
		index          = -1
		edits          = []*edit{} // key-value
		path           = []interface{}{}
		ancestors      = []interface{}{}
		ancestorsSlice = [][]interface{}{}
	)
	// these algorithm must be simple!!!
	// abstract algorithm
Loop:
	for {
		index++

		isLeaving := (len(keys) == index)
		var (
			key       interface{} // string for structs or int for slices
			node      interface{} // ast.Node or can be anything
			nodeSlice []interface{}
		)
		isEdited := (isLeaving && len(edits) != 0)

		if isLeaving {
			key, path = pop(path)

			node = parent
			parent, ancestors = pop(ancestors)
			nodeSlice = parentSlice
			parentSlice, ancestorsSlice = popNodeSlice(ancestorsSlice)

			if isEdited {
				prevInSlice = inSlice
				editOffset := 0
				for _, edit := range edits {
					if inSlice {
						if isNilNode(edit.Value) {
							nodeSlice = removeNodeByIndex(nodeSlice, edit.Key.(int)-editOffset)
							editOffset++
						} else {
							nodeSlice[edit.Key.(int)-editOffset] = edit.Value
						}
					} else {
						var isConvertMap bool
						// check if edit.Value implements ast.Node or []ast.Node.
						if !isSlice(edit.Value) {
							if !isStructNode(edit.Value) {
								isConvertMap = true
							}
						} else {
							// check if edit.value slice is ast.nodes
							ev := reflect.ValueOf(edit.Value)
							for i := 0; i < ev.Len(); i++ {
								if !isStructNode(ev.Index(i).Interface()) {
									isConvertMap = true
									break
								}
							}
						}
						if !isConvertMap {
							node = updateNodeField(node, edit.Key.(string), edit.Value)
						} else {
							// non-node needs convert to map
							if todoNode, err := convertMap(node); err != nil {
								panic(err)
							} else {
								todoNode[edit.Key.(string)] = edit.Value
								node = todoNode
							}
						}
					}
				}
			}
			index, keys, edits, inSlice, sstack = sstack.Index, sstack.Keys, sstack.Edits, sstack.inSlice, sstack.Prev
		} else {
			// get key & value
			if inSlice {
				key = index
			} else if !isNilNode(parent) {
				key = getFieldValue(keys, index)
			}
			// get node
			var tmp interface{}
			if !isNilNode(parent) {
				tmp = parent
			} else if len(parentSlice) != 0 {
				tmp = parentSlice
			} else {
				node, nodeSlice = newRoot, []interface{}{}
			}
			if tmp != nil {
				fieldValue := getFieldValue(tmp, key)
				switch {
				case isNode(fieldValue):
					node = fieldValue.(ast.Node)
				case isSlice(fieldValue):
					nodeSlice = toSliceInterfaces(fieldValue)
				}
			}

			if isNilNode(node) && len(nodeSlice) == 0 {
				continue
			}

			if !inSlice {
				if !isNilNode(parent) {
					path = append(path, key)
				}
			} else {
				if len(parentSlice) != 0 {
					path = append(path, key)
				}
			}
		}

		// get result from visitFn for a node if set
		var result interface{}
		resultIsUndefined := true
		if !isNilNode(node) {
			// Note that since user can potentially return a non-ast.Node from visit functions.
			// if not exist map type for node, nodes implement ast.Node
			parentConcrete, _ := parent.(ast.Node)
			ancestorsConcrete := []ast.Node{}
			for _, ancestor := range ancestors {
				if ancestorConcrete, ok := ancestor.(ast.Node); ok {
					ancestorsConcrete = append(ancestorsConcrete, ancestorConcrete)
				} else {
					ancestorsConcrete = append(ancestorsConcrete, nil) // map for nil
				}
			}

			var kind string
			switch tmp := node.(type) {
			case map[string]interface{}:
				kind = tmp["Kind"].(string)
			case ast.Node:
				kind = tmp.GetKind()
			}

			visitFn := GetVisitFn(visitorOpts, kind, isLeaving)
			if visitFn != nil {
				p := VisitFuncParams{
					Node:      node,
					Key:       key,
					Parent:    parentConcrete,
					Path:      path,
					Ancestors: ancestorsConcrete,
				}
				var action string
				switch action, result = visitFn(p); action {
				case ActionBreak:
					break Loop
				case ActionSkip:
					if !isLeaving {
						_, path = pop(path)
						continue
					}
				case ActionUpdate:
					resultIsUndefined = false
					edits = append(edits, &edit{
						Key:   key,
						Value: result,
					})
					if !isLeaving {
						if isNode(result) {
							node = result
						} else {
							_, path = pop(path)
							continue
						}
					}
				}
			}
		}

		// collect back edits on the way out
		if resultIsUndefined && isEdited {
			if !prevInSlice {
				edits = append(edits, &edit{
					Key:   key,
					Value: node,
				})
			} else {
				edits = append(edits, &edit{
					Key:   key,
					Value: nodeSlice,
				})
			}
		}
		if !isLeaving {
			// add to stack
			prevStack := sstack
			sstack = &stack{
				inSlice: inSlice,
				Index:   index,
				Keys:    keys,
				Edits:   edits,
				Prev:    prevStack,
			}

			// replace keys
			keys, index, edits = []interface{}{}, -1, []*edit{}
			if len(nodeSlice) > 0 {
				inSlice = true
				keys = append(keys, nodeSlice...)
			} else {
				inSlice = false
				if !isNilNode(node) {
					kind := node.(ast.Node).GetKind()
					for _, m := range visitorKeys[kind] {
						keys = append(keys, m)
					}
				}
			}
			ancestors = append(ancestors, parent)
			parent = node
			ancestorsSlice = append(ancestorsSlice, parentSlice)
			parentSlice = nodeSlice
		}

		// loop guard
		if sstack == nil {
			break Loop
		}
	}
	if len(edits) != 0 {
		result = edits[len(edits)-1].Value
	}
	return result
}

func pop(a []interface{}) (interface{}, []interface{}) {
	if len(a) == 0 {
		return nil, nil
	}
	return a[len(a)-1], a[:len(a)-1]
}

func popNodeSlice(a [][]interface{}) ([]interface{}, [][]interface{}) {
	if len(a) == 0 {
		return nil, nil
	}
	return a[len(a)-1], a[:len(a)-1]
}

func removeNodeByIndex(a []interface{}, pos int) []interface{} {
	if pos < 0 || pos >= len(a) {
		return a
	}
	return append(a[:pos], a[pos+1:]...)
}

func convertMap(src interface{}) (dest map[string]interface{}, err error) {
	if src == nil {
		return
	}
	var bts []byte
	if bts, err = json.Marshal(src); err != nil {
		return
	}
	if err = json.Unmarshal(bts, &dest); err != nil {
		return
	}
	return
}

// get value by key from struct | slice | map | wrap(prev)
// when obj type is struct, the key's type must be string
// ... slice, ... int
// ... map, ... any type. But the type satisfies map's key definition(feature: compare...)
func getFieldValue(obj interface{}, key interface{}) interface{} {
	var value reflect.Value
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.Struct:
		value = val.FieldByName(key.(string))
	case reflect.Map:
		value = val.MapIndex(reflect.ValueOf(key))
	case reflect.Slice:
		if index, ok := key.(int); !ok {
			return nil
		} else if index >= 0 || val.Len() > index {
			value = val.Index(index)
		}
	}
	if !value.IsValid() {
		return nil
	}
	return value.Interface()
}

// currently only supports update struct field value
func updateNodeField(src interface{}, targetName string, target interface{}) interface{} {
	var isPtr bool
	srcVal := reflect.ValueOf(src)
	// verify condition
	if srcVal.Kind() == reflect.Ptr {
		isPtr = true
		srcVal = srcVal.Elem()
	}
	targetVal := reflect.ValueOf(target)
	if srcVal.Kind() != reflect.Struct {
		return src
	}
	srcFieldValue := srcVal.FieldByName(targetName)
	if !srcFieldValue.IsValid() || srcFieldValue.Kind() != targetVal.Kind() {
		return src
	}

	if srcFieldValue.CanSet() {
		if srcFieldValue.Kind() == reflect.Slice {
			items := reflect.MakeSlice(srcFieldValue.Type(), targetVal.Len(), targetVal.Len())
			for index := 0; index < items.Len(); index++ {
				tmp := targetVal.Index(index).Interface()
				items.Index(index).Set(reflect.ValueOf(tmp))
			}
			srcFieldValue.Set(items)
		} else {
			srcFieldValue.Set(targetVal)
		}
	}
	if isPtr {
		return srcVal.Addr().Interface()
	}
	return srcVal.Interface()
}

func toSliceInterfaces(src interface{}) []interface{} {
	var list []interface{}
	value := reflect.ValueOf(src)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Slice {
		return nil
	}
	for index := 0; index < value.Len(); index++ {
		list = append(list, value.Index(index).Interface())
	}
	return list
}

func isSlice(value interface{}) bool {
	if value == nil {
		return false
	}
	typ := reflect.TypeOf(value)
	if typ.Kind() == reflect.Slice {
		return true
	}
	return false
}

func isStructNode(node interface{}) bool {
	if node == nil {
		return false
	}
	value := reflect.ValueOf(node)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() == reflect.Struct {
		_, ok := node.(ast.Node)
		return ok
	}
	return false
}

// notice: type: Named, List or NonNull maybe map type
// and it can't be asserted to ast.Node
func isNode(node interface{}) bool {
	if node == nil {
		return false
	}
	val := reflect.ValueOf(node)
	if !val.IsValid() {
		return false
	}
	switch val.Kind() {
	case reflect.Map:
		return true
	case reflect.Ptr:
		val = val.Elem()
	}
	_, ok := node.(ast.Node)
	return ok
}

func isNilNode(node interface{}) bool {
	if node == nil {
		return true
	}
	val := reflect.ValueOf(node)
	if !val.IsValid() {
		return true
	}
	switch val.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice:
		return val.IsNil()
	case reflect.Bool:
		return node.(bool)
	}
	return false
}

// VisitInParallel Creates a new visitor instance which delegates to many visitors to run in
// parallel. Each visitor will be visited for each node before moving on.
//
// If a prior visitor edits a node, no following visitors will see that node.
func VisitInParallel(visitorOptsSlice ...*VisitorOptions) *VisitorOptions {
	skipping := map[int]interface{}{}

	return &VisitorOptions{
		Enter: func(p VisitFuncParams) (string, interface{}) {
			for i, visitorOpts := range visitorOptsSlice {
				if _, ok := skipping[i]; !ok {
					node, ok := p.Node.(ast.Node)
					if !ok {
						continue
					}
					kind := node.GetKind()
					fn := GetVisitFn(visitorOpts, kind, false)
					if fn != nil {
						action, result := fn(p)
						if action == ActionSkip {
							skipping[i] = node
						} else if action == ActionBreak {
							skipping[i] = ActionBreak
						} else if action == ActionUpdate {
							return ActionUpdate, result
						}
					}
				}
			}
			return ActionNoChange, nil
		},
		Leave: func(p VisitFuncParams) (string, interface{}) {
			for i, visitorOpts := range visitorOptsSlice {
				skippedNode, ok := skipping[i]
				if !ok {
					if node, ok := p.Node.(ast.Node); ok {
						kind := node.GetKind()
						fn := GetVisitFn(visitorOpts, kind, true)
						if fn != nil {
							action, result := fn(p)
							if action == ActionBreak {
								skipping[i] = ActionBreak
							} else if action == ActionUpdate {
								return ActionUpdate, result
							}
						}
					}
				} else if skippedNode == p.Node {
					delete(skipping, i)
				}
			}
			return ActionNoChange, nil
		},
	}
}

// VisitWithTypeInfo Creates a new visitor instance which maintains a provided TypeInfo instance
// along with visiting visitor.
func VisitWithTypeInfo(ttypeInfo typeInfo.TypeInfoI, visitorOpts *VisitorOptions) *VisitorOptions {
	return &VisitorOptions{
		Enter: func(p VisitFuncParams) (string, interface{}) {
			if node, ok := p.Node.(ast.Node); ok {
				ttypeInfo.Enter(node)
				fn := GetVisitFn(visitorOpts, node.GetKind(), false)
				if fn != nil {
					action, result := fn(p)
					if action == ActionUpdate {
						ttypeInfo.Leave(node)
						if isNode(result) {
							if result, ok := result.(ast.Node); ok {
								ttypeInfo.Enter(result)
							}
						}
					}
					return action, result
				}
			}
			return ActionNoChange, nil
		},
		Leave: func(p VisitFuncParams) (string, interface{}) {
			action := ActionNoChange
			var result interface{}
			if node, ok := p.Node.(ast.Node); ok {
				fn := GetVisitFn(visitorOpts, node.GetKind(), true)
				if fn != nil {
					action, result = fn(p)
				}
				ttypeInfo.Leave(node)
			}
			return action, result
		},
	}
}

// GetVisitFn Given a visitor instance, if it is leaving or not, and a node kind, return
// the function the visitor runtime should call.
// priority [high->low] in VisitorOptions:
// KindFuncMap{Kind> {Leave, Enter}} > {Leave, Enter} > {EnterKindMap, LeaveKindMap}
func GetVisitFn(visitorOpts *VisitorOptions, kind string, isLeaving bool) VisitFunc {
	if visitorOpts == nil {
		return nil
	}
	if kindVisitor, ok := visitorOpts.KindFuncMap[kind]; ok {
		if !isLeaving && kindVisitor.Kind != nil {
			// { Kind() {} }
			return kindVisitor.Kind
		} else if isLeaving {
			// { Kind: { leave() {} } }
			return kindVisitor.Leave
		} else {
			// { Kind: { enter() {} } }
			return kindVisitor.Enter
		}
	}
	if isLeaving {
		// { leave() {} }
		if genericVisitor := visitorOpts.Leave; genericVisitor != nil {
			return genericVisitor
		}
		if specificKindVisitor, ok := visitorOpts.LeaveKindMap[kind]; ok {
			// { leave: { Kind() {} } }
			return specificKindVisitor
		}

	} else {
		// { enter() {} }
		if genericVisitor := visitorOpts.Enter; genericVisitor != nil {
			return genericVisitor
		}
		if specificKindVisitor, ok := visitorOpts.EnterKindMap[kind]; ok {
			// { enter: { Kind() {} } }
			return specificKindVisitor
		}
	}
	return nil
}
