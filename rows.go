package joydb

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/leengari/joydb/internal/domain/data"
	"github.com/leengari/joydb/internal/executor"
)

// Rows iterates a materialized query result.
type Rows struct {
	columns  []string
	metadata []executor.ColumnMetadata
	data     []data.Row
	cursor   int
	colIndex map[string]int
	err      error
}

func newRows(result *executor.Result) *Rows {
	index := make(map[string]int, len(result.Columns))
	for i, column := range result.Columns {
		index[strings.ToLower(column)] = i
	}
	return &Rows{
		columns:  append([]string(nil), result.Columns...),
		metadata: append([]executor.ColumnMetadata(nil), result.Metadata...),
		data:     result.Rows,
		cursor:   -1,
		colIndex: index,
	}
}

func (r *Rows) Next() bool {
	if r == nil || r.cursor+1 >= len(r.data) {
		return false
	}
	r.cursor++
	return true
}

func (r *Rows) Scan(dest ...interface{}) error {
	if r == nil || r.cursor < 0 || r.cursor >= len(r.data) {
		return fmt.Errorf("joydb: Scan called without a current row")
	}
	values := r.data[r.cursor].Values
	if len(dest) != len(values) {
		return fmt.Errorf("joydb: Scan destination count %d does not match column count %d", len(dest), len(values))
	}
	for i := range dest {
		if err := assignValue(dest[i], values[i]); err != nil {
			return fmt.Errorf("joydb: scan column %q: %w", r.columns[i], err)
		}
	}
	return nil
}

func (r *Rows) StructScan(dest interface{}) error {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() || value.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("joydb: StructScan destination must be a non-nil struct pointer")
	}
	if r.cursor < 0 || r.cursor >= len(r.data) {
		return fmt.Errorf("joydb: StructScan called without a current row")
	}
	structValue := value.Elem()
	structType := structValue.Type()
	fields := make(map[string]int)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Tag.Get("joydb")
		if name == "" {
			name = field.Name
		}
		fields[normalizeName(name)] = i
	}
	for columnIndex, column := range r.columns {
		fieldIndex, ok := fields[normalizeName(column)]
		if !ok {
			continue
		}
		if err := assignReflectValue(structValue.Field(fieldIndex), r.data[r.cursor].Values[columnIndex]); err != nil {
			return fmt.Errorf("joydb: scan column %q: %w", column, err)
		}
	}
	return nil
}

func (r *Rows) Columns() []string { return append([]string(nil), r.columns...) }
func (r *Rows) Close() error      { return nil }
func (r *Rows) Err() error        { return r.err }

func (r *Rows) Value(column string) interface{} {
	if r == nil || r.cursor < 0 || r.cursor >= len(r.data) {
		return nil
	}
	index, ok := r.colIndex[strings.ToLower(column)]
	if !ok || index >= len(r.data[r.cursor].Values) {
		return nil
	}
	return r.data[r.cursor].Values[index]
}

func (r *Rows) String(column string) string {
	value, _ := toString(r.Value(column))
	return value
}

func (r *Rows) Int(column string) int64 {
	value, _ := toInt64(r.Value(column))
	return value
}

func (r *Rows) Float(column string) float64 {
	value, _ := toFloat64(r.Value(column))
	return value
}

func (r *Rows) Bool(column string) bool {
	value, _ := toBool(r.Value(column))
	return value
}

func (r *Rows) IsNull(column string) bool { return r.Value(column) == nil }

func assignValue(dest, source interface{}) error {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	return assignReflectValue(value.Elem(), source)
}

func assignReflectValue(dest reflect.Value, source interface{}) error {
	if !dest.CanSet() {
		return fmt.Errorf("destination cannot be set")
	}
	if source == nil {
		dest.Set(reflect.Zero(dest.Type()))
		return nil
	}
	if dest.Kind() == reflect.Interface {
		dest.Set(reflect.ValueOf(source))
		return nil
	}
	switch dest.Kind() {
	case reflect.String:
		value, err := toString(source)
		if err != nil {
			return err
		}
		dest.SetString(value)
	case reflect.Bool:
		value, err := toBool(source)
		if err != nil {
			return err
		}
		dest.SetBool(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := toInt64(source)
		if err != nil {
			return err
		}
		dest.SetInt(value)
	case reflect.Float32, reflect.Float64:
		value, err := toFloat64(source)
		if err != nil {
			return err
		}
		dest.SetFloat(value)
	default:
		sourceValue := reflect.ValueOf(source)
		if sourceValue.Type().AssignableTo(dest.Type()) {
			dest.Set(sourceValue)
			return nil
		}
		return fmt.Errorf("unsupported destination type %s", dest.Type())
	}
	return nil
}

func toString(value interface{}) (string, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	if value == nil {
		return "", nil
	}
	return fmt.Sprint(value), nil
}

func toInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to integer", value)
	}
}

func toFloat64(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	case string:
		return strconv.ParseFloat(typed, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float", value)
	}
}

func toBool(value interface{}) (bool, error) {
	if typed, ok := value.(bool); ok {
		return typed, nil
	}
	if text, ok := value.(string); ok {
		return strconv.ParseBool(text)
	}
	return false, fmt.Errorf("cannot convert %T to bool", value)
}

func normalizeName(name string) string {
	var builder strings.Builder
	for _, character := range name {
		if character == '_' || character == '.' || unicode.IsSpace(character) {
			continue
		}
		builder.WriteRune(unicode.ToLower(character))
	}
	return builder.String()
}
