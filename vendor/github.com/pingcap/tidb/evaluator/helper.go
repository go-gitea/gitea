package evaluator

import (
	"strconv"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/util/types"
)

var (
	// CurrentTimestamp is the keyword getting default value for datetime and timestamp type.
	CurrentTimestamp  = "CURRENT_TIMESTAMP"
	currentTimestampL = "current_timestamp"
	// ZeroTimestamp shows the zero datetime and timestamp.
	ZeroTimestamp = "0000-00-00 00:00:00"
)

var (
	errDefaultValue = errors.New("invalid default value")
)

// GetTimeValue gets the time value with type tp.
func GetTimeValue(ctx context.Context, v interface{}, tp byte, fsp int) (interface{}, error) {
	return getTimeValue(ctx, v, tp, fsp)
}

func getTimeValue(ctx context.Context, v interface{}, tp byte, fsp int) (interface{}, error) {
	value := mysql.Time{
		Type: tp,
		Fsp:  fsp,
	}

	defaultTime, err := getSystemTimestamp(ctx)
	if err != nil {
		return nil, errors.Trace(err)
	}

	switch x := v.(type) {
	case string:
		upperX := strings.ToUpper(x)
		if upperX == CurrentTimestamp {
			value.Time = defaultTime
		} else if upperX == ZeroTimestamp {
			value, _ = mysql.ParseTimeFromNum(0, tp, fsp)
		} else {
			value, err = mysql.ParseTime(x, tp, fsp)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
	case *ast.ValueExpr:
		switch x.Kind() {
		case types.KindString:
			value, err = mysql.ParseTime(x.GetString(), tp, fsp)
			if err != nil {
				return nil, errors.Trace(err)
			}
		case types.KindInt64:
			value, err = mysql.ParseTimeFromNum(x.GetInt64(), tp, fsp)
			if err != nil {
				return nil, errors.Trace(err)
			}
		case types.KindNull:
			return nil, nil
		default:
			return nil, errors.Trace(errDefaultValue)
		}
	case *ast.FuncCallExpr:
		if x.FnName.L == currentTimestampL {
			return CurrentTimestamp, nil
		}
		return nil, errors.Trace(errDefaultValue)
	case *ast.UnaryOperationExpr:
		// support some expression, like `-1`
		v, err := Eval(ctx, x)
		if err != nil {
			return nil, errors.Trace(err)
		}
		ft := types.NewFieldType(mysql.TypeLonglong)
		xval, err := types.Convert(v, ft)
		if err != nil {
			return nil, errors.Trace(err)
		}

		value, err = mysql.ParseTimeFromNum(xval.(int64), tp, fsp)
		if err != nil {
			return nil, errors.Trace(err)
		}
	default:
		return nil, nil
	}

	return value, nil
}

// IsCurrentTimeExpr returns whether e is CurrentTimeExpr.
func IsCurrentTimeExpr(e ast.ExprNode) bool {
	x, ok := e.(*ast.FuncCallExpr)
	if !ok {
		return false
	}
	return x.FnName.L == currentTimestampL
}

func getSystemTimestamp(ctx context.Context) (time.Time, error) {
	value := time.Now()

	if ctx == nil {
		return value, nil
	}

	// check whether use timestamp varibale
	sessionVars := variable.GetSessionVars(ctx)
	if v, ok := sessionVars.Systems["timestamp"]; ok {
		if v != "" {
			timestamp, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return time.Time{}, errors.Trace(err)
			}

			if timestamp <= 0 {
				return value, nil
			}

			return time.Unix(timestamp, 0), nil
		}
	}

	return value, nil
}
