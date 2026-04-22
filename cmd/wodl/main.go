package main

import (
	"fmt"
	"html/template"
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

	// Services
	authService := services.NewAuthService(userRepo, jwtService)
	liftService := services.NewLiftService(liftRepo, liftLogRepo)
	workoutService := services.NewWorkoutService(workoutRepo, workoutResultRepo)
	sessionService := services.NewSessionService(sessionRepo, workoutRepo)

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
	}

	tmpl := template.Must(
		template.New("").Funcs(funcMap).ParseFS(templates.FS, "*.html"),
	)

	// Handlers
	authHandler := handlers.NewAuthHandler(authService, tmpl)
	dashHandler := handlers.NewDashboardHandler(liftService, workoutService, sessionService, tmpl)
	liftHandler := handlers.NewLiftHandler(liftService, tmpl)
	workoutHandler := handlers.NewWorkoutHandler(workoutService, liftService, tmpl)
	sessionHandler := handlers.NewSessionHandler(sessionService, workoutService, tmpl)

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

		r.Get("/api/search", dashHandler.Search)
		r.Get("/api/1rm-calc", liftHandler.Calc1RM)
	})

	log.Printf("Starting WODL on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
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
