package app_error

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   error
}

func NewAppError(code int, message string, e error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Error:   e,
	}
}
