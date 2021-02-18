package codegen

import (
	"fmt"

	"github.com/99designs/gqlgen/codegen/config"
)

func (b *builder) buildTypes() map[string]*config.TypeReference {
	ret := map[string]*config.TypeReference{}
	for _, ref := range b.Binder.References {
		processType(ret, ref)
	}
	return ret
}

func processType(ret map[string]*config.TypeReference, ref *config.TypeReference) {
	key := ref.UniquenessKey()
	if existing, found := ret[key]; found {
		// Simplistic check of content which is obviously different.
		existingGQL := fmt.Sprintf("%v", existing.GQL)
		newGQL := fmt.Sprintf("%v", ref.GQL)
		if existingGQL != newGQL {
			panic(fmt.Sprintf("non-unique key \"%s\", trying to replace %s with %s", key, existingGQL, newGQL))
		}
	}
	ret[key] = ref

	if ref.IsSlice() {
		processType(ret, ref.Elem())
	}
}
