// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package evaluator

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/util/types"
)

func convertToTime(arg types.Datum, tp byte) (d types.Datum, err error) {
	f := types.NewFieldType(tp)
	f.Decimal = mysql.MaxFsp

	d, err = arg.ConvertTo(f)
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}

	if d.Kind() == types.KindNull {
		return d, nil
	}

	if d.Kind() != types.KindMysqlTime {
		err = errors.Errorf("need time type, but got %T", d.GetValue())
		d.SetNull()
		return d, err
	}
	return d, nil
}

func convertToDuration(arg types.Datum, fsp int) (d types.Datum, err error) {
	f := types.NewFieldType(mysql.TypeDuration)
	f.Decimal = fsp

	d, err = arg.ConvertTo(f)
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}

	if d.Kind() == types.KindNull {
		d.SetNull()
		return d, nil
	}

	if d.Kind() != types.KindMysqlDuration {
		err = errors.Errorf("need duration type, but got %T", d.GetValue())
		d.SetNull()
		return d, err
	}
	return d, nil
}

func builtinDate(args []types.Datum, _ context.Context) (types.Datum, error) {
	return convertToTime(args[0], mysql.TypeDate)
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_day
// day is a synonym for DayOfMonth
func builtinDay(args []types.Datum, ctx context.Context) (types.Datum, error) {
	return builtinDayOfMonth(args, ctx)
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_hour
func builtinHour(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToDuration(args[0], mysql.MaxFsp)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	h := int64(d.GetMysqlDuration().Hour())
	d.SetInt64(h)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_minute
func builtinMinute(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToDuration(args[0], mysql.MaxFsp)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	m := int64(d.GetMysqlDuration().Minute())
	d.SetInt64(m)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_second
func builtinSecond(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToDuration(args[0], mysql.MaxFsp)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	s := int64(d.GetMysqlDuration().Second())
	d.SetInt64(s)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_microsecond
func builtinMicroSecond(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToDuration(args[0], mysql.MaxFsp)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	m := int64(d.GetMysqlDuration().MicroSecond())
	d.SetInt64(m)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_month
func builtinMonth(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	i := int64(0)
	if t.IsZero() {
		d.SetInt64(i)
		return d, nil
	}
	i = int64(t.Month())
	d.SetInt64(i)
	return d, nil
}

func builtinNow(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	// TODO: if NOW is used in stored function or trigger, NOW will return the beginning time
	// of the execution.
	fsp := 0
	if len(args) == 1 && args[0].Kind() != types.KindNull {
		if fsp, err = checkFsp(args[0]); err != nil {
			d.SetNull()
			return d, errors.Trace(err)
		}
	}

	t := mysql.Time{
		Time: time.Now(),
		Type: mysql.TypeDatetime,
		// set unspecified for later round
		Fsp: mysql.UnspecifiedFsp,
	}

	tr, err := t.RoundFrac(int(fsp))
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}
	d.SetMysqlTime(tr)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_dayname
func builtinDayName(args []types.Datum, ctx context.Context) (types.Datum, error) {
	d, err := builtinWeekDay(args, ctx)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}
	weekday := d.GetInt64()
	if (weekday < 0) || (weekday >= int64(len(mysql.WeekdayNames))) {
		d.SetNull()
		return d, errors.Errorf("no name for invalid weekday: %d.", weekday)
	}
	d.SetString(mysql.WeekdayNames[weekday])
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_dayofmonth
func builtinDayOfMonth(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	// TODO: some invalid format like 2000-00-00 will return 0 too.
	d, err = convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		d.SetInt64(int64(0))
		return d, nil
	}

	d.SetInt64(int64(t.Day()))
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_dayofweek
func builtinDayOfWeek(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	d, err = convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		d.SetNull()
		// TODO: log warning or return error?
		return d, nil
	}

	// 1 is Sunday, 2 is Monday, .... 7 is Saturday
	d.SetInt64(int64(t.Weekday()) + 1)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_dayofyear
func builtinDayOfYear(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	t := d.GetMysqlTime()
	if t.IsZero() {
		// TODO: log warning or return error?
		d.SetNull()
		return d, nil
	}

	yd := int64(t.YearDay())
	d.SetInt64(yd)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_week
func builtinWeek(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		// TODO: log warning or return error?
		d.SetNull()
		return d, nil
	}

	// TODO: support multi mode for week
	_, week := t.ISOWeek()
	wi := int64(week)
	d.SetInt64(wi)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_weekday
func builtinWeekDay(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		// TODO: log warning or return error?
		d.SetNull()
		return d, nil
	}

	// Monday is 0, ... Sunday = 6 in MySQL
	// but in go, Sunday is 0, ... Saturday is 6
	// w will do a conversion.
	w := (int64(t.Weekday()) + 6) % 7
	d.SetInt64(w)
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_weekofyear
func builtinWeekOfYear(args []types.Datum, ctx context.Context) (types.Datum, error) {
	// WeekOfYear is equivalent to to Week(date, 3)
	d := types.Datum{}
	d.SetInt64(3)
	return builtinWeek([]types.Datum{args[0], d}, ctx)
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_year
func builtinYear(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		d.SetInt64(0)
		return d, nil
	}

	d.SetInt64(int64(t.Year()))
	return d, nil
}

// See http://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_yearweek
func builtinYearWeek(args []types.Datum, _ context.Context) (types.Datum, error) {
	d, err := convertToTime(args[0], mysql.TypeDate)
	if err != nil || d.Kind() == types.KindNull {
		d.SetNull()
		return d, errors.Trace(err)
	}

	// No need to check type here.
	t := d.GetMysqlTime()
	if t.IsZero() {
		d.SetNull()
		// TODO: log warning or return error?
		return d, nil
	}

	// TODO: support multi mode for week
	year, week := t.ISOWeek()
	d.SetInt64(int64(year*100 + week))
	return d, nil
}

func builtinSysDate(args []types.Datum, ctx context.Context) (types.Datum, error) {
	// SYSDATE is not the same as NOW if NOW is used in a stored function or trigger.
	// But here we can just think they are the same because we don't support stored function
	// and trigger now.
	return builtinNow(args, ctx)
}

// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_curdate
func builtinCurrentDate(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	year, month, day := time.Now().Date()
	t := mysql.Time{
		Time: time.Date(year, month, day, 0, 0, 0, 0, time.Local),
		Type: mysql.TypeDate, Fsp: 0}
	d.SetMysqlTime(t)
	return d, nil
}

// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_curtime
func builtinCurrentTime(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	fsp := 0
	if len(args) == 1 && args[0].Kind() != types.KindNull {
		if fsp, err = checkFsp(args[0]); err != nil {
			d.SetNull()
			return d, errors.Trace(err)
		}
	}
	d.SetString(time.Now().Format("15:04:05.000000"))
	return convertToDuration(d, fsp)
}

// See https://dev.mysql.com/doc/refman/5.7/en/date-and-time-functions.html#function_extract
func builtinExtract(args []types.Datum, _ context.Context) (d types.Datum, err error) {
	unit := args[0].GetString()
	vd := args[1]

	if vd.Kind() == types.KindNull {
		d.SetNull()
		return d, nil
	}

	f := types.NewFieldType(mysql.TypeDatetime)
	f.Decimal = mysql.MaxFsp
	val, err := vd.ConvertTo(f)
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}
	if val.Kind() == types.KindNull {
		d.SetNull()
		return d, nil
	}

	if val.Kind() != types.KindMysqlTime {
		err = errors.Errorf("need time type, but got %T", val)
		d.SetNull()
		return d, err
	}
	t := val.GetMysqlTime()
	n, err1 := mysql.ExtractTimeNum(unit, t)
	if err1 != nil {
		d.SetNull()
		return d, errors.Trace(err1)
	}
	d.SetInt64(n)
	return d, nil
}

func checkFsp(arg types.Datum) (int, error) {
	fsp, err := arg.ToInt64()
	if err != nil {
		return 0, errors.Trace(err)
	}
	if int(fsp) > mysql.MaxFsp {
		return 0, errors.Errorf("Too big precision %d specified. Maximum is 6.", fsp)
	} else if fsp < 0 {
		return 0, errors.Errorf("Invalid negative %d specified, must in [0, 6].", fsp)
	}
	return int(fsp), nil
}

func builtinDateArith(args []types.Datum, ctx context.Context) (d types.Datum, err error) {
	// Op is used for distinguishing date_add and date_sub.
	// args[0] -> Op
	// args[1] -> Date
	// args[2] -> DateArithInterval
	// health check for date and interval
	if args[1].Kind() == types.KindNull {
		d.SetNull()
		return d, nil
	}
	nodeDate := args[1]
	nodeInterval := args[2].GetInterface().(ast.DateArithInterval)
	nodeIntervalIntervalDatum := nodeInterval.Interval.GetDatum()
	if nodeIntervalIntervalDatum.Kind() == types.KindNull {
		d.SetNull()
		return d, nil
	}
	// parse date
	fieldType := mysql.TypeDate
	var resultField *types.FieldType
	switch nodeDate.Kind() {
	case types.KindMysqlTime:
		x := nodeDate.GetMysqlTime()
		if (x.Type == mysql.TypeDatetime) || (x.Type == mysql.TypeTimestamp) {
			fieldType = mysql.TypeDatetime
		}
	case types.KindString:
		x := nodeDate.GetString()
		if !mysql.IsDateFormat(x) {
			fieldType = mysql.TypeDatetime
		}
	case types.KindInt64:
		x := nodeDate.GetInt64()
		if t, err1 := mysql.ParseTimeFromInt64(x); err1 == nil {
			if (t.Type == mysql.TypeDatetime) || (t.Type == mysql.TypeTimestamp) {
				fieldType = mysql.TypeDatetime
			}
		}
	}
	if mysql.IsClockUnit(nodeInterval.Unit) {
		fieldType = mysql.TypeDatetime
	}
	resultField = types.NewFieldType(fieldType)
	resultField.Decimal = mysql.MaxFsp
	value, err := nodeDate.ConvertTo(resultField)
	if err != nil {
		d.SetNull()
		return d, ErrInvalidOperation.Gen("DateArith invalid args, need date but get %T", nodeDate)
	}
	if value.Kind() == types.KindNull {
		d.SetNull()
		return d, ErrInvalidOperation.Gen("DateArith invalid args, need date but get %v", value.GetValue())
	}
	if value.Kind() != types.KindMysqlTime {
		d.SetNull()
		return d, ErrInvalidOperation.Gen("DateArith need time type, but got %T", value.GetValue())
	}
	result := value.GetMysqlTime()
	// parse interval
	var interval string
	if strings.ToLower(nodeInterval.Unit) == "day" {
		day, err2 := parseDayInterval(*nodeIntervalIntervalDatum)
		if err2 != nil {
			d.SetNull()
			return d, ErrInvalidOperation.Gen("DateArith invalid day interval, need int but got %T", nodeIntervalIntervalDatum.GetString())
		}
		interval = fmt.Sprintf("%d", day)
	} else {
		if nodeIntervalIntervalDatum.Kind() == types.KindString {
			interval = fmt.Sprintf("%v", nodeIntervalIntervalDatum.GetString())
		} else {
			ii, err := nodeIntervalIntervalDatum.ToInt64()
			if err != nil {
				d.SetNull()
				return d, errors.Trace(err)
			}
			interval = fmt.Sprintf("%v", ii)
		}
	}
	year, month, day, duration, err := mysql.ExtractTimeValue(nodeInterval.Unit, interval)
	if err != nil {
		d.SetNull()
		return d, errors.Trace(err)
	}
	op := args[0].GetInterface().(ast.DateArithType)
	if op == ast.DateSub {
		year, month, day, duration = -year, -month, -day, -duration
	}
	result.Time = result.Time.Add(duration)
	result.Time = result.Time.AddDate(int(year), int(month), int(day))
	if result.Time.Nanosecond() == 0 {
		result.Fsp = 0
	}
	d.SetMysqlTime(result)
	return d, nil
}

var reg = regexp.MustCompile(`[\d]+`)

func parseDayInterval(value types.Datum) (int64, error) {
	switch value.Kind() {
	case types.KindString:
		vs := value.GetString()
		s := strings.ToLower(vs)
		if s == "false" {
			return 0, nil
		} else if s == "true" {
			return 1, nil
		}
		value.SetString(reg.FindString(vs))
	}
	return value.ToInt64()
}
