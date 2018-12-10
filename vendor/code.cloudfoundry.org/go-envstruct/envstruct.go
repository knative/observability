package envstruct

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	indexEnvVar = 0

	tagRequired = "required"
	tagNoReport = "noreport"
)

// Unmarshaller is a type which unmarshals itself from an environment variable.
type Unmarshaller interface {
	UnmarshalEnv(v string) error
}

// Load will use the `env` tags from a struct to populate the structs values and
// perform validations.
func Load(t interface{}) error {
	val := reflect.ValueOf(t).Elem()

	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := tagProperties[indexEnvVar]
		envVal := os.Getenv(envVar)
		required := tagPropertiesContains(tagProperties, tagRequired)

		if isInvalid(envVal, required) {
			return fmt.Errorf("%s is required but was empty", envVar)
		}

		err := setField(valueField, envVal)
		if err != nil {
			return err
		}
	}

	return nil
}

// ToEnv will return a slice of strings that can be used with exec.Cmd.Env
// formatted as `ENVAR_NAME=value` for a given struct.
func ToEnv(t interface{}) []string {
	val := reflect.ValueOf(t).Elem()

	var results []string
	for i := 0; i < val.NumField(); i++ {
		valueField := val.Field(i)
		typeField := val.Type().Field(i)
		tag := typeField.Tag

		tagProperties := separateOnComma(tag.Get("env"))
		envVar := tagProperties[indexEnvVar]

		switch valueField.Kind() {
		case reflect.Slice:
			results = append(results, formatSlice(envVar, valueField))
		case reflect.Map:
			results = append(results, formatMap(envVar, valueField))
		case reflect.Struct:
			results = append(results, ToEnv(valueField.Addr().Interface())...)
		case reflect.Ptr:
			if valueField.Type() == reflect.TypeOf(&url.URL{}) {
				results = append(results, fmt.Sprintf("%s=%+v", envVar, valueField))
				continue
			}

			results = append(results, ToEnv(valueField.Interface())...)
		default:
			results = append(results, fmt.Sprintf("%s=%+v", envVar, valueField))
		}
	}

	return results
}

func tagPropertiesContains(properties []string, match string) bool {
	for _, v := range properties {
		if v == match {
			return true
		}
	}

	return false
}

func unmarshaller(v reflect.Value) (Unmarshaller, bool) {
	if unmarshaller, ok := v.Interface().(Unmarshaller); ok {
		return unmarshaller, ok
	}
	if v.CanAddr() {
		return unmarshaller(v.Addr())
	}
	return nil, false
}

func setField(value reflect.Value, input string) error {
	if input == "" &&
		(value.Kind() != reflect.Ptr) &&
		(value.Kind() != reflect.Struct) {

		return nil
	}

	if unmarshaller, ok := unmarshaller(value); ok {
		return unmarshaller.UnmarshalEnv(input)
	}
	switch value.Type() {
	case reflect.TypeOf(time.Second):
		return setDuration(value, input)
	case reflect.TypeOf(&url.URL{}):
		return setURL(value, input)
	}

	switch value.Kind() {
	case reflect.String:
		return setString(value, input)
	case reflect.Bool:
		return setBool(value, input)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return setInt(value, input)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUint(value, input)
	case reflect.Slice:
		return setSlice(value, input)
	case reflect.Map:
		return setMap(value, input)
	case reflect.Struct:
		return setStruct(value)
	case reflect.Ptr:
		return setPointerToStruct(value)
	}

	return nil
}

func separateOnComma(input string) []string {
	inputs := strings.Split(input, ",")

	for i, v := range inputs {
		inputs[i] = strings.TrimSpace(v)
	}

	return inputs
}

func isInvalid(input string, required bool) bool {
	return required && input == ""
}

func setStruct(value reflect.Value) error {
	err := Load(value.Addr().Interface())
	if err != nil {
		return err
	}

	return nil
}

func setPointerToStruct(value reflect.Value) error {
	if value.IsNil() {
		p := reflect.New(value.Type().Elem())
		value.Set(p)
	}

	err := Load(value.Interface())
	if err != nil {
		return err
	}

	return nil
}

func setDuration(value reflect.Value, input string) error {
	d, err := time.ParseDuration(input)
	if err != nil {
		return err
	}

	value.Set(reflect.ValueOf(d))

	return nil
}

func setURL(value reflect.Value, input string) error {
	u, err := url.Parse(input)
	if err != nil {
		return err
	}

	value.Set(reflect.ValueOf(u))

	return nil
}

func setString(value reflect.Value, input string) error {
	value.SetString(input)

	return nil
}

func setBool(value reflect.Value, input string) error {
	value.SetBool(input == "true" || input == "1")

	return nil
}

func setInt(value reflect.Value, input string) error {
	n, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return err
	}

	value.SetInt(int64(n))

	return nil
}

func setUint(value reflect.Value, input string) error {
	n, err := strconv.ParseUint(input, 10, 64)
	if err != nil {
		return err
	}

	value.SetUint(uint64(n))

	return nil
}

func setSlice(value reflect.Value, input string) error {
	inputs := separateOnComma(input)

	rs := reflect.MakeSlice(value.Type(), len(inputs), len(inputs))
	for i, val := range inputs {
		err := setField(rs.Index(i), val)
		if err != nil {
			return err
		}
	}

	value.Set(rs)

	return nil
}

func setMap(value reflect.Value, input string) error {
	inputs := separateOnComma(input)

	m := make(map[string]string)
	for _, i := range inputs {
		kv := strings.SplitN(i, ":", 2)

		if len(kv) == 0 {
			continue
		}

		if len(kv) < 2 {
			return fmt.Errorf("map[string]string key '%s' is missing a value", kv[0])
		}

		m[kv[0]] = kv[1]
	}

	value.Set(reflect.ValueOf(m))

	return nil
}

func formatSlice(envVar string, value reflect.Value) string {
	var parts []string
	for i := 0; i < value.Len(); i++ {
		parts = append(parts, fmt.Sprintf("%+v", value.Index(i)))
	}

	return fmt.Sprintf("%s=%+v", envVar, strings.Join(parts, ","))
}

func formatMap(envVar string, value reflect.Value) string {
	var parts []string

	keys := value.MapKeys()
	for _, k := range keys {
		v := value.MapIndex(k)

		parts = append(parts, fmt.Sprintf("%+v:%+v", k, v))
	}

	return fmt.Sprintf("%s=%+v", envVar, strings.Join(parts, ","))
}
