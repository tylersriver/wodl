package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
)

type BiometricHandler struct {
	deviceTokenService *services.DeviceTokenService
}

func NewBiometricHandler(deviceTokenService *services.DeviceTokenService) *BiometricHandler {
	return &BiometricHandler{deviceTokenService: deviceTokenService}
}

type createTokenRequest struct {
	DeviceName string `json:"device_name"`
}

type createTokenResponse struct {
	DeviceToken string `json:"device_token"`
}

// RegisterDevice creates a new device token for the authenticated user.
// POST /api/device-token (requires auth)
func (h *BiometricHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.DeviceName = "iOS Device"
	}
	if req.DeviceName == "" {
		req.DeviceName = "iOS Device"
	}

	rawToken, err := h.deviceTokenService.CreateToken(userId, req.DeviceName)
	if err != nil {
		http.Error(w, `{"error":"failed to create device token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(createTokenResponse{DeviceToken: rawToken})
}

type exchangeTokenRequest struct {
	DeviceToken string `json:"device_token"`
}

// ExchangeToken validates a device token and returns a JWT session cookie.
// POST /api/biometric-login (public)
func (h *BiometricHandler) ExchangeToken(w http.ResponseWriter, r *http.Request) {
	var req exchangeTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DeviceToken == "" {
		http.Error(w, `{"error":"device_token is required"}`, http.StatusBadRequest)
		return
	}

	jwtToken, err := h.deviceTokenService.ExchangeToken(req.DeviceToken)
	if err != nil {
		http.Error(w, `{"error":"invalid device token"}`, http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// RevokeDeviceTokens removes all device tokens for the authenticated user.
// DELETE /api/device-token (requires auth)
func (h *BiometricHandler) RevokeDeviceTokens(w http.ResponseWriter, r *http.Request) {
	userId := middleware.GetUserID(r)

	if err := h.deviceTokenService.RevokeAllTokens(userId); err != nil {
		http.Error(w, `{"error":"failed to revoke tokens"}`, http.StatusInternalServerError)
		return
	}

	// Also clear the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
