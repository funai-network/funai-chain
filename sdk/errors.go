package sdk

import "fmt"

// ErrorCode represents a standardized FunAI error code (OpenAI-compatible).
type ErrorCode string

const (
	ErrInsufficientBalance ErrorCode = "insufficient_balance"
	ErrModelNotFound       ErrorCode = "model_not_found"
	ErrNoAvailableWorker   ErrorCode = "no_available_worker"
	ErrRequestTimeout      ErrorCode = "request_timeout"
	ErrFeeTooLow           ErrorCode = "fee_too_low"
	ErrContentTagNoWorker  ErrorCode = "content_tag_no_worker"
	ErrMaxTokensExceeded   ErrorCode = "max_tokens_exceeded"
	ErrInvalidParameters   ErrorCode = "invalid_parameters"
	ErrNetworkError        ErrorCode = "network_error"
	ErrJSONParseFailed     ErrorCode = "json_parse_failed"
)

// FunAIError is a structured error type compatible with OpenAI's error format.
type FunAIError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Type    string    `json:"type"`
}

func (e *FunAIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewError creates a new FunAIError with the given code and message.
func NewError(code ErrorCode, message string) *FunAIError {
	return &FunAIError{
		Code:    code,
		Message: message,
		Type:    string(code),
	}
}

// NewErrorf creates a new FunAIError with a formatted message.
func NewErrorf(code ErrorCode, format string, args ...interface{}) *FunAIError {
	return NewError(code, fmt.Sprintf(format, args...))
}

// ErrorResponse represents the OpenAI-compatible error response envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the inner error object matching OpenAI's format.
type ErrorDetail struct {
	Message string    `json:"message"`
	Type    string    `json:"type"`
	Code    ErrorCode `json:"code"`
}

// ToResponse converts a FunAIError to an OpenAI-compatible error response.
func (e *FunAIError) ToResponse() ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Message: e.Message,
			Type:    e.Type,
			Code:    e.Code,
		},
	}
}

// httpStatusMap maps error codes to HTTP status codes for gateway/REST usage.
var httpStatusMap = map[ErrorCode]int{
	ErrInsufficientBalance: 402,
	ErrModelNotFound:       404,
	ErrNoAvailableWorker:   503,
	ErrRequestTimeout:      408,
	ErrFeeTooLow:           422,
	ErrContentTagNoWorker:  503,
	ErrMaxTokensExceeded:   400,
	ErrInvalidParameters:   400,
	ErrNetworkError:        500,
	ErrJSONParseFailed:     422,
}

// HTTPStatus returns the recommended HTTP status code for this error.
func (e *FunAIError) HTTPStatus() int {
	if code, ok := httpStatusMap[e.Code]; ok {
		return code
	}
	return 500
}
