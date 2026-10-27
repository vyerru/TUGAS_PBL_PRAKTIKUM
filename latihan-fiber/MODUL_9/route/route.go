package route

import (
	"context"
	"time"

	"MODUL_9/app/service"
	"MODUL_9/helper"
	"MODUL_9/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	Pool           *pgxpool.Pool
	JWT            *helper.JWTManager
	Permissions    *helper.PermissionSet
	AuthService    *service.AuthService
	StudentService *service.StudentService
	UserService    *service.UserService
	CourseService  *service.CourseService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	api.Get("/health", healthCheck(deps.Pool))

	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	students := api.Group("/students",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))

	perms := deps.Permissions

	students.Get("/",
		middleware.RequirePermission(perms, "student:list"),
		deps.StudentService.List)
	students.Post("/",
		middleware.RequirePermission(perms, "student:create"),
		deps.StudentService.Create)
	students.Delete("/:id",
		middleware.RequirePermission(perms, "student:delete"),
		deps.StudentService.Delete)

	students.Get("/:id", deps.StudentService.Get)
	students.Put("/:id", deps.StudentService.Replace)
	students.Patch("/:id", deps.StudentService.Patch)

	users := api.Group("/users",
	middleware.RequireJSON,
	middleware.RequireAuth(deps.JWT))

users.Get("/",
	middleware.RequirePermission(perms, "user:list"),
	deps.UserService.List)
users.Post("/",
	middleware.RequirePermission(perms, "user:create"),
	deps.UserService.Create)
users.Delete("/:id",
	middleware.RequirePermission(perms, "user:delete"),
	deps.UserService.Delete)

users.Get("/:id", deps.UserService.Get)
users.Put("/:id", deps.UserService.Replace)
users.Patch("/:id", deps.UserService.Patch)

// --- kelas dan KRS ---
courses := api.Group("/courses",
	middleware.RequireJSON,
	middleware.RequireAuth(deps.JWT))

// "/mine" didaftarkan SEBELUM "/:id". Fiber mencocokkan route sesuai
// urutan pendaftaran; bila dibalik, "mine" akan dianggap sebagai :id.
courses.Get("/", deps.CourseService.List)
courses.Get("/mine", deps.CourseService.Mine)
courses.Get("/:id", deps.CourseService.Get)
courses.Post("/:id/enroll", deps.CourseService.Enroll)
courses.Delete("/:id/enroll", deps.CourseService.Drop)
}

func healthCheck(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return helper.ServiceUnavailable("database tidak dapat dihubungi")
		}
		return helper.Success(c, fiber.StatusOK, "server dan database berjalan", nil)
	}
}