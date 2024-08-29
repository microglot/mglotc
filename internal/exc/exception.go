// © 2023 Microglot LLC
//
// SPDX-License-Identifier: Apache-2.0

package exc

import (
	"fmt"

	"gopkg.microglot.org/compiler.go/internal/idl"
)

type Exception interface {
	error
	Code() string
	Message() string
	Location() Location
}

type Location struct {
	idl.Location
	URI string
}

type exc struct {
	code     string
	message  string
	location Location
}

func (e *exc) Error() string {
	return fmt.Sprintf("%s:%d:%d -- %s: %s", e.location.URI, e.location.Line, e.location.Column, e.code, e.message)
}

func (e *exc) Code() string {
	return e.code
}

func (e *exc) Message() string {
	return e.message
}

func (e *exc) Location() Location {
	return e.location
}

type excUnwrap struct {
	Exception
	cause error
}

func (e *excUnwrap) Unwrap() error {
	return e.cause
}

func New(location Location, code string, message string) Exception {
	return &exc{
		location: location,
		message:  message,
		code:     code,
	}
}

func Wrap(location Location, code string, err error) Exception {
	if err == nil {
		return nil
	}
	if e, ok := err.(Exception); ok {
		return &excUnwrap{
			Exception: New(location, code, e.Message()),
			cause:     e,
		}
	}
	return &excUnwrap{
		cause:     err,
		Exception: New(location, code, err.Error()),
	}
}

func WrapUnknown(location Location, err error) Exception {
	return Wrap(location, CodeUnknownFatal, err)
}
