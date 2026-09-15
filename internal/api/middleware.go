package api

import (
	"context"
	"net/http"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

type contextKey string

const userContextKey contextKey = "user"

// AuthMiddleware valida la sesión y, si es válida, inyecta el usuario en el contexto.
func AuthMiddleware(auth *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.CookieName())
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := auth.ValidateSession(r.Context(), cookie.Value)
			if err != nil || user == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// userFromContext extrae el usuario autenticado del contexto.
func userFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok
}
