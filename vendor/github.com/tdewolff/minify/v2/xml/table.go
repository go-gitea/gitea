package xml

// Entities are all named character entities.
var EntitiesMap = map[string][]byte{
	"apos": []byte("'"),
	"gt":   []byte(">"),
	"quot": []byte("\""),
}
