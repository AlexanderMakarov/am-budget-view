package bankdownload

import (
	"errors"
	"fmt"
	"net"
	"net/url"
)

// UserFacingError is an error with a short user-visible message and an optional hint.
type UserFacingError struct {
	Message string
	Hint    string
}

func (e UserFacingError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("%s (%s)", e.Message, e.Hint)
	}
	return e.Message
}

// ErrUnauthorized indicates the Ameria Business API rejected the session cookie.
var ErrUnauthorized = errors.New("ameria business api unauthorized")

type unauthorizedError struct {
	body string
	op   string
}

func (e *unauthorizedError) Error() string {
	if e.op != "" {
		return fmt.Sprintf("AmeriaBank Business API 401 Unauthorized on %s", e.op)
	}
	return "AmeriaBank Business API 401 Unauthorized"
}

func (e *unauthorizedError) Is(target error) bool {
	return target == ErrUnauthorized
}

// ErrRefreshUnauthorized indicates the refresh token is no longer valid.
var ErrRefreshUnauthorized = errors.New("ameria business refresh unauthorized")

// ErrRefreshFailed indicates cookie refresh failed for a non-auth reason.
var ErrRefreshFailed = errors.New("ameria business refresh failed")

type myAmeriaHTTPError struct {
	statusCode int
	body       string
}

func (e *myAmeriaHTTPError) Error() string {
	body := e.body
	if body == "" {
		body = "(empty)"
	}
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Sprintf("MyAmeria API returned HTTP %d. Response: %s", e.statusCode, body)
}

// ErrMyAmeriaInvalidResponse indicates the MyAmeria API returned a non-JSON body,
// usually because the Authorization token expired mid-session.
var ErrMyAmeriaInvalidResponse = errors.New("myameria response is not valid json")

// ameriaBusinessHTTPError is a non-2xx (and non-401) response from the AmeriaBank Business API.
// The status code is carried explicitly so MapError can branch on it without substring matching.
type ameriaBusinessHTTPError struct {
	statusCode int
	op         string
	body       string
}

func (e *ameriaBusinessHTTPError) Error() string {
	return fmt.Sprintf("AmeriaBank Business %s request failed: HTTP %d: %s", e.op, e.statusCode, e.body)
}

// MapError converts internal/download errors into user-facing messages and hints.
func MapError(err error) UserFacingError {
	if err == nil {
		return UserFacingError{}
	}

	var ufe UserFacingError
	if errors.As(err, &ufe) {
		return ufe
	}

	var myAmeriaHTTP *myAmeriaHTTPError
	if errors.As(err, &myAmeriaHTTP) {
		switch myAmeriaHTTP.statusCode {
		case 401, 403:
			return UserFacingError{
				Message: fmt.Sprintf("MyAmeria API returned HTTP %d", myAmeriaHTTP.statusCode),
				Hint: "Authorization token expired or invalid. Copy a fresh Bearer token from DevTools " +
					"(Network → request to ob.myameria.am → Authorization header) right before download.",
			}
		default:
			return UserFacingError{
				Message: myAmeriaHTTP.Error(),
				Hint:    "The bank service may be temporarily unavailable. Try again later.",
			}
		}
	}

	if errors.Is(err, ErrMyAmeriaInvalidResponse) {
		return UserFacingError{
			Message: "Unexpected response from MyAmeria API (not JSON)",
			Hint:    "Authorization token likely expired — paste a fresh one and try again.",
		}
	}

	var ameriaHTTP *ameriaBusinessHTTPError
	if errors.As(err, &ameriaHTTP) {
		switch ameriaHTTP.statusCode {
		case 401:
			return UserFacingError{
				Message: "AmeriaBank Business API returned 401 Unauthorized",
				Hint: "Log in again at https://business.myameria.am and copy a fresh cookie " +
					"from a request to gateway-businessmyameria.ameriabank.am.",
			}
		case 403:
			return UserFacingError{
				Message: "AmeriaBank Business API returned 403 Forbidden",
				Hint:    "Your account may not have access to this resource. Verify you are logged into the correct business account.",
			}
		case 500, 502, 503, 504:
			return UserFacingError{
				Message: "AmeriaBank Business server error",
				Hint:    "The bank service may be temporarily unavailable. Try again later.",
			}
		default:
			return UserFacingError{
				Message: ameriaHTTP.Error(),
				Hint:    "See application logs for details.",
			}
		}
	}

	if errors.Is(err, ErrIncompleteCookie) {
		return UserFacingError{
			Message: "AmeriaBank Business cookie is incomplete",
			Hint: "Copy the full Cookie header from DevTools → Network → any request to " +
				"gateway-businessmyameria.ameriabank.am. It must include RefreshToken and AccessToken " +
				"(not just TS* cookies from the Application tab).",
		}
	}

	if errors.Is(err, ErrRefreshUnauthorized) {
		return UserFacingError{
			Message: "AmeriaBank Business session refresh failed (401)",
			Hint: "RefreshToken expired or invalid. Log in again at https://business.myameria.am " +
				"and copy a fresh Cookie header from a request to gateway-businessmyameria.ameriabank.am.",
		}
	}

	if errors.Is(err, ErrRefreshFailed) {
		return UserFacingError{
			Message: "AmeriaBank Business session refresh failed",
			Hint:    "Check your network connection and try again. If the problem persists, log in again at https://business.myameria.am.",
		}
	}

	if errors.Is(err, ErrUnauthorized) {
		return UserFacingError{
			Message: "AmeriaBank Business API returned 401 Unauthorized",
			Hint: "Your session cookie may have expired. Log in again at https://business.myameria.am " +
				"and update the cookie in bank download settings.",
		}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return UserFacingError{
				Message: "Request to AmeriaBank Business timed out",
				Hint:    "The bank server did not respond in time. Check your connection and try again.",
			}
		}
		return UserFacingError{
			Message: "Could not reach AmeriaBank Business",
			Hint:    "Check your internet connection and try again.",
		}
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return UserFacingError{
				Message: "Network request timed out",
				Hint:    "Check your internet connection and try again.",
			}
		}
		return UserFacingError{
			Message: "Network error while contacting AmeriaBank Business",
			Hint:    "Check your internet connection and try again.",
		}
	}

	return UserFacingError{
		Message: err.Error(),
		Hint:    "See application logs for details.",
	}
}
