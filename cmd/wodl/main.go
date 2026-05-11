package main

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/auth"
	"github.com/tyler/wodl/internal/infrastructure/db/sqlite"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
	"github.com/tyler/wodl/internal/interface/web/handlers"
	"github.com/tyler/wodl/internal/interface/web/static"
	"github.com/tyler/wodl/internal/interface/web/templates"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "wodl-dev-secret-change-in-production"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "wodl.db"
	}

	// Ensure DB directory exists (for volume mounts)
	if dir := filepath.Dir(dbPath); dir != "." {
		os.MkdirAll(dir, 0755)
	}

	// Infrastructure
	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	jwtService := auth.NewJWTService(jwtSecret)

	// Repositories
	userRepo := sqlite.NewUserRepository(db)
	liftRepo := sqlite.NewLiftRepository(db)
	liftLogRepo := sqlite.NewLiftLogRepository(db)
	workoutRepo := sqlite.NewWorkoutRepository(db)
	workoutResultRepo := sqlite.NewWorkoutResultRepository(db)
	sessionRepo := sqlite.NewSessionRepository(db)
	sessionLogRepo := sqlite.NewSessionLogRepository(db)

	// Services
	authService := services.NewAuthService(userRepo, jwtService)
	liftService := services.NewLiftService(liftRepo, liftLogRepo)
	workoutService := services.NewWorkoutService(workoutRepo, workoutResultRepo)
	sessionService := services.NewSessionService(sessionRepo, workoutRepo, sessionLogRepo)
	quickLogService := services.NewQuickLogService(liftService, workoutService, sessionService)

	// Templates
	funcMap := template.FuncMap{
		"deref": func(f *float64) float64 {
			if f == nil {
				return 0
			}
			return *f
		},
		"derefInt": func(i *int) int {
			if i == nil {
				return 0
			}
			return *i
		},
		"inc":  func(i int) int { return i + 1 },
		"dict": dictFunc,
	}

	tmpl := template.Must(
		template.New("").Funcs(funcMap).ParseFS(templates.FS, "*.html"),
	)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, tmpl)
	dashHandler := handlers.NewDashboardHandler(liftService, workoutService, sessionService, tmpl)
	liftHandler := handlers.NewLiftHandler(liftService, tmpl)
	workoutHandler := handlers.NewWorkoutHandler(workoutService, liftService, tmpl)
	sessionHandler := handlers.NewSessionHandler(sessionService, workoutService, liftService, tmpl)
	quickLogHandler := handlers.NewQuickLogHandler(quickLogService, liftService, workoutService, tmpl)

	// Router
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(methodOverride)

	// Public routes
	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)
	r.Get("/register", authHandler.RegisterPage)
	r.Post("/register", authHandler.Register)

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// PWA assets — public so browsers can fetch them before login.
	staticFS, err := fs.Sub(static.FS, ".")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", pwaFileServer(http.FS(staticFS))))
	r.Get("/manifest.webmanifest", pwaAssetHandler(static.FS, "manifest.webmanifest", "application/manifest+json"))
	r.Get("/sw.js", pwaAssetHandler(static.FS, "sw.js", "application/javascript"))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtService))

		r.Get("/", dashHandler.Dashboard)
		r.Post("/logout", authHandler.Logout)

		r.Get("/lifts", liftHandler.List)
		r.Post("/lifts", liftHandler.Create)
		r.Get("/lifts/{id}", liftHandler.Detail)
		r.Put("/lifts/{id}", liftHandler.Update)
		r.Delete("/lifts/{id}", liftHandler.Delete)
		r.Post("/lifts/{id}/logs", liftHandler.CreateLog)
		r.Delete("/lifts/{id}/logs/{logId}", liftHandler.DeleteLog)

		r.Get("/workouts", workoutHandler.List)
		r.Post("/workouts", workoutHandler.Create)
		r.Get("/workouts/{id}", workoutHandler.Detail)
		r.Put("/workouts/{id}", workoutHandler.Update)
		r.Delete("/workouts/{id}", workoutHandler.Delete)
		r.Post("/workouts/{id}/results", workoutHandler.CreateResult)

		r.Get("/sessions", sessionHandler.List)
		r.Post("/sessions", sessionHandler.Create)
		r.Get("/sessions/{id}", sessionHandler.Detail)
		r.Put("/sessions/{id}", sessionHandler.Update)
		r.Delete("/sessions/{id}", sessionHandler.Delete)
		r.Post("/sessions/{id}/logs", sessionHandler.CreateLog)
		r.Delete("/sessions/{id}/logs/{logId}", sessionHandler.DeleteLog)

		r.Get("/quick-log", quickLogHandler.Page)
		r.Post("/quick-log", quickLogHandler.Submit)

		r.Get("/api/search", dashHandler.Search)
		r.Get("/api/1rm-calc", liftHandler.Calc1RM)
	})

	log.Printf("Starting WODL on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// dictFunc builds a map from alternating key/value template args so partials
// can be invoked with named fields — e.g. {{template "x" (dict "K" v)}}.
func dictFunc(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 {
		return nil, errors.New("dict: odd number of args")
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key must be string, got %T", values[i])
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// pwaAssetHandler serves a single embedded asset at its top-level URL. Used
// for `/sw.js` and `/manifest.webmanifest`, which must live at the site root
// so the service worker can claim the whole `/` scope.
func pwaAssetHandler(efs fs.FS, name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(efs, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		// Required so the service worker can control the entire site scope.
		if name == "sw.js" {
			w.Header().Set("Service-Worker-Allowed", "/")
		}
		w.Write(data)
	}
}

// pwaFileServer wraps http.FileServer so 404s render as plain text rather than
// the FileServer's HTML, which would otherwise be cached by the service worker
// as a navigation fallback.
func pwaFileServer(root http.FileSystem) http.Handler {
	fileServer := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fileServer.ServeHTTP(w, r)
	})
}

// methodOverride allows HTML forms to use PUT/DELETE via _method field.
func methodOverride(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if method := r.FormValue("_method"); method != "" {
				r.Method = method
			}
		}
		next.ServeHTTP(w, r)
	})
}
