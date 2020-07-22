package kinds

const (
	// Name
	Name = "Name"

	// Document
	Document            = "Document"
	OperationDefinition = "OperationDefinition"
	VariableDefinition  = "VariableDefinition"
	Variable            = "Variable"
	SelectionSet        = "SelectionSet"
	Field               = "Field"
	Argument            = "Argument"

	// Fragments
	FragmentSpread     = "FragmentSpread"
	InlineFragment     = "InlineFragment"
	FragmentDefinition = "FragmentDefinition"

	// Values
	IntValue     = "IntValue"
	FloatValue   = "FloatValue"
	StringValue  = "StringValue"
	BooleanValue = "BooleanValue"
	EnumValue    = "EnumValue"
	ListValue    = "ListValue"
	ObjectValue  = "ObjectValue"
	ObjectField  = "ObjectField"

	// Directives
	Directive = "Directive"

	// Types
	Named   = "Named"   // previously NamedType
	List    = "List"    // previously ListType
	NonNull = "NonNull" // previously NonNull

	// Type System Definitions
	SchemaDefinition        = "SchemaDefinition"
	OperationTypeDefinition = "OperationTypeDefinition"

	// Types Definitions
	ScalarDefinition      = "ScalarDefinition" // previously ScalarTypeDefinition
	ObjectDefinition      = "ObjectDefinition" // previously ObjectTypeDefinition
	FieldDefinition       = "FieldDefinition"
	InputValueDefinition  = "InputValueDefinition"
	InterfaceDefinition   = "InterfaceDefinition" // previously InterfaceTypeDefinition
	UnionDefinition       = "UnionDefinition"     // previously UnionTypeDefinition
	EnumDefinition        = "EnumDefinition"      // previously EnumTypeDefinition
	EnumValueDefinition   = "EnumValueDefinition"
	InputObjectDefinition = "InputObjectDefinition" // previously InputObjectTypeDefinition

	// Types Extensions
	TypeExtensionDefinition = "TypeExtensionDefinition"

	// Directive Definitions
	DirectiveDefinition = "DirectiveDefinition"
)
