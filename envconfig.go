// Copyright (c) 2013 Kelsey Hightower. All rights reserved.
// Use of this source code is governed by the MIT License that can be found in
// the LICENSE file.

package envconfig

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// ErrInvalidSpecification indicates that a specification is of the wrong type.
var ErrInvalidSpecification = errors.New("invalid specification must be a struct")

// A ParseError occurs when an environment variable cannot be converted to
// the type required by a struct field during assignment.
type ParseError struct {
	KeyName   string
	FieldName string
	TypeName  string
	Value     string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("envconfig.Process: assigning %[1]s to %[2]s: converting '%[3]s' to type %[4]s", e.KeyName, e.FieldName, e.Value, e.TypeName)
}

// A RequiredError occurs when a required environment variable cannot be found
// on the environment.
type RequiredError struct {
	KeyName string
}

func (e *RequiredError) Error() string {
	return fmt.Sprintf("envconfig.Process: required key %[1]s not found", e.KeyName)
}

type MultiError []error

func (e MultiError) Error() string {
	str := "[\n"
	for _, err := range []error(e) {
		str += " - " + err.Error() + "\n"
	}
	return str + "]"
}

// Process parses the environment and loads the contents into the matching
// elements inside the provided spec.
func Process(prefix string, spec interface{}) error {
	s := reflect.ValueOf(spec).Elem()
	if s.Kind() != reflect.Struct {
		return ErrInvalidSpecification
	}
	errors := make([]error, 0)
	typeOfSpec := s.Type()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		if f.CanSet() {
			fieldName := typeOfSpec.Field(i).Tag.Get("envconfig")
			if fieldName == "" {
				continue
			}
			key := strings.ToUpper(fmt.Sprintf("%s_%s", prefix, fieldName))
			value := os.Getenv(key)

			def := typeOfSpec.Field(i).Tag.Get("default")
			if def != "" && value == "" {
				value = def
			}

			req := typeOfSpec.Field(i).Tag.Get("required")
			if value == "" && f.Kind() != reflect.Struct {
				if req == "true" {
					errors = append(errors, &RequiredError{
						KeyName: key,
					})
				}
				continue
			}

			switch f.Kind() {
			case reflect.String:
				f.SetString(value)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				intValue, err := strconv.ParseInt(value, 0, f.Type().Bits())
				if err != nil {
					errors = append(errors, &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					})
					continue
				}
				f.SetInt(intValue)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint64:
				uintValue, err := strconv.ParseUint(value, 0, f.Type().Bits())
				if err != nil {
					errors = append(errors, &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					})
					continue
				}
				f.SetUint(uintValue)
			case reflect.Struct:
				structPtr := reflect.New(f.Type()).Interface()
				if err := Process(key, structPtr); err != nil {
					return err
				}
				f.Set(reflect.ValueOf(structPtr).Elem())
			case reflect.Bool:
				boolValue, err := strconv.ParseBool(value)
				if err != nil {
					errors = append(errors, &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					})
					continue
				}
				f.SetBool(boolValue)
			case reflect.Float32, reflect.Float64:
				floatValue, err := strconv.ParseFloat(value, f.Type().Bits())
				if err != nil {
					errors = append(errors, &ParseError{
						KeyName:   key,
						FieldName: fieldName,
						TypeName:  f.Type().String(),
						Value:     value,
					})
					continue
				}
				f.SetFloat(floatValue)
			case reflect.Ptr:
				if t := f.Type().Elem(); t.Kind() == reflect.Struct && t.PkgPath() == "net/url" && t.Name() == "URL" {
					v, err := url.Parse(value)
					if err == nil {
						f.Set(reflect.ValueOf(v))
					}
				}

			}
		}
	}

	if len(errors) > 0 {
		return MultiError(errors)
	}
	return nil
}
