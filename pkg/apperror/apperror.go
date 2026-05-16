package apperror

import "net/http"

type Apperror struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Apperror) Error() string {
	return e.Message
}

func BadRequest(msg string) error {
	return &Apperror{
		Code:    http.StatusBadRequest,
		Message: msg,
	}
}

func Unauthorized(msg string) error {
	return &Apperror{
		Code:    http.StatusUnauthorized,
		Message: msg,
	}
}

func Forbidden(msg string) error {
	return &Apperror{
		Code:    http.StatusForbidden,
		Message: msg,
	}
}

func NotFound(msg string) error {
	return &Apperror{
		Code:    http.StatusNotFound,
		Message: msg,
	}
}

func Internal(msg string) error {
	return &Apperror{
		Code:    http.StatusInternalServerError,
		Message: msg,
	}
}

func UnprocessableEntity(msg string) error {
	return &Apperror{
		Code:    http.StatusUnprocessableEntity,
		Message: msg,
	}
}

func MethodNotAllowed(msg string) error {
	return &Apperror{
		Code:    http.StatusMethodNotAllowed, // 405
		Message: msg,
	}
}

func Conflict(msg string) error {
	return &Apperror{
		Code:    http.StatusConflict, // 409
		Message: msg,
	}
}

// 410 Gone / 401: Bisa buat OTP yang udah kadaluwarsa
func Expired(msg string) error {
	return &Apperror{
		Code:    http.StatusGone, // Atau 401 tergantung selera flow lu
		Message: msg,
	}
}

// 429 Too Many Requests: Penting buat limitasi OTP biar gak dispam
func TooManyRequests(msg string) error {
	return &Apperror{
		Code:    http.StatusTooManyRequests,
		Message: msg,
	}
}
