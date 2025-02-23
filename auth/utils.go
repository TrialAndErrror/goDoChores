package auth

import (
	"errors"
	"github.com/go-chi/jwtauth/v5"
	"net/http"
)

func GetCurrentUserID(r *http.Request) (uint, error) {
	// Extract token claims from the request context.
	_, claims, err := jwtauth.FromContext(r.Context())
	if err != nil {
		return 0, errors.New("missing token claims")
	}

	// Retrieve the user_id claim.
	// Note: Claims from JWT are often unmarshaled as float64.
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("invalid user id")
	}
	return uint(userIDFloat), nil
}
