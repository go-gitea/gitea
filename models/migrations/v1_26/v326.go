package v1_26

import "xorm.io/xorm"

func AddJobMaxParallel(x *xorm.Engine) error {
	type ActionRunJob struct {
		MaxParallel int
	}

	return x.Sync(new(ActionRunJob))
}
