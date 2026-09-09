// Package httpx defines the unified HTTP response envelope and error model
// shared by every handler: {code, message, data, request_id}.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope is the wire format of every JSON response.
type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

// Business codes. 0 means success; everything else maps to an HTTP status.
const (
	CodeOK           = 0
	CodeBadRequest   = 40000
	CodeUnauthorized = 40100
	CodeForbidden    = 40300
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 50000
)

// AppError is an error that knows how to render itself.
type AppError struct {
	Code    int
	Status  int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func newErr(code, status int, msg string) *AppError {
	return &AppError{Code: code, Status: status, Message: msg}
}

func BadRequest(msg string) *AppError   { return newErr(CodeBadRequest, http.StatusBadRequest, msg) }
func Unauthorized(msg string) *AppError { return newErr(CodeUnauthorized, http.StatusUnauthorized, msg) }
func Forbidden(msg string) *AppError    { return newErr(CodeForbidden, http.StatusForbidden, msg) }
func NotFound(msg string) *AppError     { return newErr(CodeNotFound, http.StatusNotFound, msg) }
func Conflict(msg string) *AppError     { return newErr(CodeConflict, http.StatusConflict, msg) }
func Internal(err error) *AppError {
	return &AppError{Code: CodeInternal, Status: http.StatusInternalServerError, Message: "internal error", Err: err}
}

// OK writes a success envelope.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: CodeOK, Message: "ok", Data: data, RequestID: RequestID(c)})
}

// Created writes a 201 success envelope.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Code: CodeOK, Message: "ok", Data: data, RequestID: RequestID(c)})
}

// Fail writes an error envelope. Unknown errors become 500 without leaking details.
func Fail(c *gin.Context, err error) {
	var ae *AppError
	if !errors.As(err, &ae) {
		ae = Internal(err)
	}
	_ = c.Error(err) // surface to the logging middleware
	c.AbortWithStatusJSON(ae.Status, Envelope{Code: ae.Code, Message: ae.Message, RequestID: RequestID(c)})
}

const requestIDKey = "request_id"

// RequestID returns the request id set by the middleware, if any.
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// SetRequestID stores the request id on the context.
func SetRequestID(c *gin.Context, id string) { c.Set(requestIDKey, id) }
