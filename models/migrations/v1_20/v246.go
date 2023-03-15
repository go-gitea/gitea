package v1_20

import "xorm.io/xorm"

func AddVersionToActionRunner(x *xorm.Engine) error {

	type ActionRunner struct {
		Version string `xorm:"VARCHAR(64)"` // the version of act_runner
	}

	return x.Sync(new(ActionRunner))
}
