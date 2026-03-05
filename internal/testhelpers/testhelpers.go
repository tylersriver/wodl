package testhelpers

import (
	"database/sql"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tyler/wodl/internal/application/services"
	"github.com/tyler/wodl/internal/infrastructure/auth"
	"github.com/tyler/wodl/internal/infrastructure/db/sqlite"
	"github.com/tyler/wodl/internal/infrastructure/middleware"
	"github.com/tyler/wodl/internal/interface/web/handlers"
	"github.com/tyler/wodl/internal/interface/web/templates"
)

type TestApp struct {
	Server         *httptest.Server
	DB             *sql.DB
	AuthService    *services.AuthService
	LiftService    *services.LiftService
	WorkoutService *services.WorkoutService
	JWTService     *auth.JWTService
}

func NewTestApp(t *testing.T) *TestApp {
	t.Helper()

	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	jwtService := auth.NewJWTService("test-secret")

	userRepo := sqlite.NewUserRepository(db)
	liftRepo := sqlite.NewLiftRepository(db)
	liftLogRepo := sqlite.NewLiftLogRepository(db)
	workoutRepo := sqlite.NewWorkoutRepository(db)
	workoutResultRepo := sqlite.NewWorkoutResultRepository(db)

	authService := services.NewAuthService(userRepo, jwtService)
	liftService := services.NewLiftService(liftRepo, liftLogRepo)
	workoutService := services.NewWorkoutService(workoutRepo, workoutResultRepo)

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

	authHandler := handlers.NewAuthHandler(authService, tmpl)
	dashHandler := handlers.NewDashboardHandler(liftService, workoutService, tmpl)
	liftHandler := handlers.NewLiftHandler(liftService, tmpl)
	workoutHandler := handlers.NewWorkoutHandler(workoutService, tmpl)

	r := chi.NewRouter()
	r.Use(methodOverride)

	r.Get("/login", authHandler.LoginPage)
	r.Post("/login", authHandler.Login)
	r.Get("/register", authHandler.RegisterPage)
	r.Post("/register", authHandler.Register)

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

		r.Get("/api/1rm-calc", liftHandler.Calc1RM)
	})

	server := httptest.NewServer(r)

	t.Cleanup(func() {
		server.Close()
		db.Close()
	})

	return &TestApp{
		Server:         server,
		DB:             db,
		AuthService:    authService,
		LiftService:    liftService,
		WorkoutService: workoutService,
		JWTService:     jwtService,
	}
}

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
