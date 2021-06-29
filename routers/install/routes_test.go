package install

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRoutes(t *testing.T) {
	routes := Routes()
	assert.NotNil(t, routes)
	assert.Len(t, routes.R.Routes(), 1)
	assert.EqualValues(t, "/", routes.R.Routes()[0].Pattern)
	assert.Nil(t, routes.R.Routes()[0].SubRoutes)
	assert.Len(t, routes.R.Routes()[0].Handlers, 2)
}
