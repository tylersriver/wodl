package handlers

import (
	"html/template"
	"net/http"
	"time"

	"github.com/tyler/wodl/internal/application/command"
	"github.com/tyler/wodl/internal/application/services"
)

// sessionCookieMaxAge is the cookie lifetime in seconds. Modern browsers cap
// cookie lifetime at ~400 days regardless of what we set, so this is the
// practical ceiling for a "never expires" session.
const sessionCookieMaxAge = 400 * 24 * 60 * 60

type AuthHandler struct {
	authService *services.AuthService
	templates   *template.Template
}

func NewAuthHandler(authService *services.AuthService, templates *template.Template) *AuthHandler {
	return &AuthHandler{authService: authService, templates: templates}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "login.html", map[string]interface{}{
		"Error": r.URL.Query().Get("error"),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cmd := &command.LoginCommand{
		Email:    r.FormValue("email"),
		Password: r.FormValue("password"),
	}

	result, err := h.authService.Login(cmd)
	if err != nil {
		h.templates.ExecuteTemplate(w, "login.html", map[string]interface{}{
			"Error": "Invalid email or password",
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionCookieMaxAge,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "register.html", nil)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	cmd := &command.RegisterUserCommand{
		Email:       r.FormValue("email"),
		Password:    r.FormValue("password"),
		DisplayName: r.FormValue("display_name"),
	}

	result, err := h.authService.Register(cmd)
	if err != nil {
		h.templates.ExecuteTemplate(w, "register.html", map[string]interface{}{
			"Error": err.Error(),
		})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    result.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionCookieMaxAge,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
