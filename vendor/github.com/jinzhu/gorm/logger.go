package gorm

import (
	"database/sql/driver"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"time"
	"unicode"
)

var (
	defaultLogger            = Logger{log.New(os.Stdout, "\r\n", 0)}
	sqlRegexp                = regexp.MustCompile(`\?`)
	numericPlaceHolderRegexp = regexp.MustCompile(`\$\d+`)
)

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

var LogFormatter = func(values ...interface{}) (messages []interface{}) {
	if len(values) > 1 {
		var (
			sql             string
			formattedValues []string
			level           = values[0]
			currentTime     = "\n\033[33m[" + NowFunc().Format("2006-01-02 15:04:05") + "]\033[0m"
			source          = fmt.Sprintf("\033[35m(%v)\033[0m", values[1])
		)

		messages = []interface{}{source, currentTime}

		if level == "sql" {
			// duration
			messages = append(messages, fmt.Sprintf(" \033[36;1m[%.2fms]\033[0m ", float64(values[2].(time.Duration).Nanoseconds()/1e4)/100.0))
			// sql

			for _, value := range values[4].([]interface{}) {
				indirectValue := reflect.Indirect(reflect.ValueOf(value))
				if indirectValue.IsValid() {
					value = indirectValue.Interface()
					if t, ok := value.(time.Time); ok {
						if t.IsZero() {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", "0000-00-00 00:00:00"))
						} else {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format("2006-01-02 15:04:05")))
						}
					} else if b, ok := value.([]byte); ok {
						if str := string(b); isPrintable(str) {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", str))
						} else {
							formattedValues = append(formattedValues, "'<binary>'")
						}
					} else if r, ok := value.(driver.Valuer); ok {
						if value, err := r.Value(); err == nil && value != nil {
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
						} else {
							formattedValues = append(formattedValues, "NULL")
						}
					} else {
						switch value.(type) {
						case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
							formattedValues = append(formattedValues, fmt.Sprintf("%v", value))
						default:
							formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
						}
					}
				} else {
					formattedValues = append(formattedValues, "NULL")
				}
			}

			// differentiate between $n placeholders or else treat like ?
			if numericPlaceHolderRegexp.MatchString(values[3].(string)) {
				sql = values[3].(string)
				for index, value := range formattedValues {
					placeholder := fmt.Sprintf(`\$%d([^\d]|$)`, index+1)
					sql = regexp.MustCompile(placeholder).ReplaceAllString(sql, value+"$1")
				}
			} else {
				formattedValuesLength := len(formattedValues)
				for index, value := range sqlRegexp.Split(values[3].(string), -1) {
					sql += value
					if index < formattedValuesLength {
						sql += formattedValues[index]
					}
				}
			}

			messages = append(messages, sql)
			messages = append(messages, fmt.Sprintf(" \n\033[36;31m[%v]\033[0m ", strconv.FormatInt(values[5].(int64), 10)+" rows affected or returned "))
		} else {
			messages = append(messages, "\033[31;1m")
			messages = append(messages, values[2:]...)
			messages = append(messages, "\033[0m")
		}
	}

	return
}

type logger interface {
	Print(v ...interface{})
}

// LogWriter log writer interface
type LogWriter interface {
	Println(v ...interface{})
}

// Logger default logger
type Logger struct {
	LogWriter
}

// Print format & print log
func (logger Logger) Print(values ...interface{}) {
	logger.Println(LogFormatter(values...)...)
}
