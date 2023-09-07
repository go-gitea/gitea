package setting

import (
	 "secsys/gout-transformation/pkg/transstruct"
)
import (
	 "os"
)
import (
	 "github.com/stretchr/testify/assert"
)
import (
	 "testing"
)

func FuzzTest_getPostgreSQLConnectionString(XVl []byte) int {
	t := &testing.T{}
	_ = t
	var skippingTableDriven bool
	_, skippingTableDriven = os.LookupEnv("SKIPPING_TABLE_DRIVEN")
	_ = skippingTableDriven
	transstruct.SetFuzzData(XVl)
	FDG_FuzzGlobal()

	tests := []struct {
		Host	string
		Port	string
		User	string
		Passwd	string
		Name	string
		Param	string
		SSLMode	string
		Output	string
	}{
		{
			Host:		transstruct.GetString("/tmp/pg.sock"),
			Port:		"4321",
			User:		transstruct.GetString("testuser"),
			Passwd:		transstruct.GetString("space space !#$%^^%^```-=?="),
			Name:		transstruct.GetString("gitea"),
			Param:		transstruct.GetString(""),
			SSLMode:	transstruct.GetString("false"),
			Output:		"postgres://testuser:space%20space%20%21%23$%25%5E%5E%25%5E%60%60%60-=%3F=@:5432/giteasslmode=false&host=/tmp/pg.sock",
		},
		{
			Host:		transstruct.GetString("localhost"),
			Port:		"1234",
			User:		transstruct.GetString("pgsqlusername"),
			Passwd:		transstruct.GetString("I love Gitea!"),
			Name:		transstruct.GetString("gitea"),
			Param:		transstruct.GetString(""),
			SSLMode:	transstruct.GetString("true"),
			Output:		"postgres://pgsqlusername:I%20love%20Gitea%21@localhost:5432/giteasslmode=true",
		},
	}

	for _, test := range tests {
		connStr := getPostgreSQLConnectionString(test.Host, test.User, test.Passwd, test.Name, test.Param, test.SSLMode)
		assert.Equal(t, test.Output, connStr)
	}

	return 1
}

func FDG_FuzzGlobal() {

}
