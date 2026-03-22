package public

import (
	"mime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentDisposition(t *testing.T) {
	table := []struct {
		disposition ContentDispositionType
		filename    string
		header      string
	}{
		{disposition: ContentDispositionInline, filename: "test.txt", header: "inline; filename=test.txt"},
		{disposition: ContentDispositionInline, filename: "test❌.txt", header: "inline; filename=test_.txt; filename*=utf-8''test%E2%9D%8C.txt"},
		{disposition: ContentDispositionInline, filename: "test ❌.txt", header: "inline; filename=\"test _.txt\"; filename*=utf-8''test%20%E2%9D%8C.txt"},
	}

	for _, entry := range table {
		t.Run(string(entry.disposition)+"_"+entry.filename, func(t *testing.T) {
			encoded := EncodeContentDisposition(entry.disposition, entry.filename)
			assert.Equal(t, entry.header, encoded)
			disposition, params, err := mime.ParseMediaType(encoded)
			require.NoError(t, err)
			assert.Equal(t, string(entry.disposition), disposition)
			assert.Equal(t, entry.filename, params["filename"])
		})
	}
}
