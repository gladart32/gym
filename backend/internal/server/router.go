package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/profile"
	"github.com/gladart32/gym/internal/programs"
	"github.com/gladart32/gym/internal/transport/http/handler"
	"github.com/gladart32/gym/internal/transport/http/middleware"
)

func NewRouter(log *slog.Logger, cfg *config.Config, health *handler.Health, tasksH *handler.Tasks, authH *handler.Auth, authService *auth.Service, profileH *handler.Profile, profileService *profile.Service, exercisesH *handler.Exercises, programsH *handler.Programs, programsService *programs.Service) *gin.Engine {
	if cfg.Env == "dev" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.Recover(log),
	)

	r.GET("/healthz", health.Liveness)
	r.GET("/ready", health.Readiness)
	r.Static("/media", "media")

	api := r.Group("/api/v1")
	{
		api.POST("/tasks/example", tasksH.EnqueueExample)
		api.POST("/tasks/db-cleanup", tasksH.EnqueueDBCleanup)

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authH.Register)
			authGroup.POST("/login", authH.Login)
			authGroup.POST("/refresh", authH.Refresh)
			authGroup.POST("/verify-email", authH.VerifyEmail)
			authGroup.POST("/forgot-password", authH.ForgotPassword)
			authGroup.POST("/reset-password", authH.ResetPassword)
			authGroup.POST("/resend-verification", authH.ResendVerification)

			protected := authGroup.Group("", middleware.Auth(authService))
			{
				protected.POST("/logout", authH.Logout)
				protected.GET("/sessions", authH.ListSessions)
				protected.DELETE("/sessions/:id", authH.RevokeSessionByID)
				protected.POST("/sessions/revoke-all", authH.RevokeAllSessions)
				protected.POST("/password", authH.ChangePassword)
			}
		}

		profileGroup := api.Group("/profile", middleware.Auth(authService))
		{
			profileGroup.GET("", profileH.Get)
			profileGroup.PUT("", profileH.Update)
			profileGroup.GET("/measurements", profileH.ListMeasurements)
			profileGroup.POST("/measurements", profileH.UpsertMeasurement)
			profileGroup.DELETE("/measurements/:id", profileH.DeleteMeasurement)
			profileGroup.GET("/equipment", profileH.Equipment)
			profileGroup.PUT("/equipment", profileH.ReplaceEquipment)
		}

		exercisesGroup := api.Group("/exercises", middleware.Auth(authService))
		{
			exercisesGroup.GET("", exercisesH.List)
			exercisesGroup.GET("/filters", exercisesH.Filters)
			exercisesGroup.GET("/:id", exercisesH.Detail)
			exercisesGroup.POST("/:id/favorite", exercisesH.AddFavorite)
			exercisesGroup.DELETE("/:id/favorite", exercisesH.RemoveFavorite)
		}

		programsGroup := api.Group("/programs", middleware.Auth(authService))
		{
			programsGroup.GET("", programsH.List)
			programsGroup.GET("/:id", programsH.Detail)
			programsGroup.POST("", programsH.Create)
			programsGroup.PUT("/:id", programsH.Update)
			programsGroup.DELETE("/:id", programsH.Delete)
			programsGroup.POST("/:id/copy", programsH.Copy)
		}
	}

	return r
}
