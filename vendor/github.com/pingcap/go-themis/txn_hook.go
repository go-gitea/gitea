package themis

// Hooks for debugging and testing
type fnHook func(txn *themisTxn, ctx interface{}) (bypass bool, ret interface{}, err error)

var emptyHookFn = func(txn *themisTxn, ctx interface{}) (bypass bool, ret interface{}, err error) {
	return true, nil, nil
}

type txnHook struct {
	afterChoosePrimaryAndSecondary fnHook
	beforePrewritePrimary          fnHook
	beforePrewriteLockClean        fnHook
	beforePrewriteSecondary        fnHook
	beforeCommitPrimary            fnHook
	beforeCommitSecondary          fnHook
	onSecondaryOccursLock          fnHook
	onPrewriteRow                  fnHook
	onTxnSuccess                   fnHook
	onTxnFailed                    fnHook
}

func newHook() *txnHook {
	return &txnHook{
		afterChoosePrimaryAndSecondary: emptyHookFn,
		beforePrewritePrimary:          emptyHookFn,
		beforePrewriteLockClean:        emptyHookFn,
		beforePrewriteSecondary:        emptyHookFn,
		beforeCommitPrimary:            emptyHookFn,
		beforeCommitSecondary:          emptyHookFn,
		onSecondaryOccursLock:          emptyHookFn,
		onPrewriteRow:                  emptyHookFn,
		onTxnSuccess:                   emptyHookFn,
		onTxnFailed:                    emptyHookFn,
	}
}
