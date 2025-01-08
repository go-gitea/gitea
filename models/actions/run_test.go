package actions

import (
	db_model "code.gitea.io/gitea/models/db"
	"testing"
)

func TestDeleteRunByIDs(t *testing.T) {
	tests := []struct{
		name string
		mock func(m *db_model.Engine)

	}
}
