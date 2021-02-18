package ast

type Operation string

const (
	Query        Operation = "query"
	Mutation     Operation = "mutation"
	Subscription Operation = "subscription"
)

type OperationDefinition struct {
	Operation           Operation
	Name                string
	VariableDefinitions VariableDefinitionList
	Directives          DirectiveList
	SelectionSet        SelectionSet
	Position            *Position `dump:"-"`
}

type VariableDefinition struct {
	Variable     string
	Type         *Type
	DefaultValue *Value
	Position     *Position `dump:"-"`

	// Requires validation
	Definition *Definition
	Used       bool `dump:"-"`
}
