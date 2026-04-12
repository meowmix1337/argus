package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"

	"github.com/meowmix1337/argus/backend/internal/config"
	"github.com/meowmix1337/argus/backend/internal/events"
	"github.com/meowmix1337/argus/backend/internal/handler"
	"github.com/meowmix1337/argus/backend/internal/httpclient"
	"github.com/meowmix1337/argus/backend/internal/middleware"
	"github.com/meowmix1337/argus/backend/internal/repository"
	"github.com/meowmix1337/argus/backend/internal/response"
	"github.com/meowmix1337/argus/backend/internal/service"
	"github.com/meowmix1337/argus/backend/internal/validate"
)

// Server holds the HTTP router and all dependencies.
type Server struct {
	router *chi.Mux
	cfg    *config.Config
	db     *sqlx.DB
	encSvc *service.EncryptionService // nil means no encryption
}

// New creates a new Server with all services, handlers, and routes registered.
func New(cfg *config.Config, db *sqlx.DB, encSvc *service.EncryptionService) *Server {
	s := &Server{
		router: chi.NewRouter(),
		cfg:    cfg,
		db:     db,
		encSvc: encSvc,
	}
	s.setupRoutes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	r := s.router

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.CORS(s.cfg.CORSOrigin))
	r.Use(middleware.Logging)

	// Shared dependencies
	rawHTTP := &http.Client{Timeout: 30 * time.Second}
	hc := httpclient.New(rawHTTP)
	cache := service.NewCacheService()
	v := validate.New()

	// Services
	weatherSvc := service.NewWeatherService(hc, cache, s.cfg.Latitude, s.cfg.Longitude)
	newsSvc := service.NewNewsService(hc, s.cfg.GNewsAPIKey, cache)
	watchlistRepo := repository.NewSQLiteStocksWatchlistRepository(s.db)
	stocksSvc := service.NewStocksService(hc, s.cfg.FinnhubAPIKey, cache, watchlistRepo)
	taskRepo := repository.NewSQLiteTaskRepository(s.db)
	tasksSvc := service.NewTasksService(taskRepo)
	billsRepo := repository.NewSQLiteBillsRepository(s.db)
	billPaymentsRepo := repository.NewSQLiteBillPaymentsRepository(s.db)
	billsSvc := service.NewBillsService(billsRepo, billPaymentsRepo)
	settingsRepo := repository.NewSQLiteUserSettingsRepository(s.db)
	settingsSvc := service.NewUserSettingsService(settingsRepo, s.encSvc)
	calendarSvc := service.NewCalendarService(hc, s.cfg.ICSCalendarURL, cache, s.cfg.Timezone, settingsSvc)
	labelRepo := repository.NewSQLiteTaskLabelsRepository(s.db)
	labelsSvc := service.NewTaskLabelsService(labelRepo)
	sunriseSvc := service.NewSunriseService(hc, cache, s.cfg.Latitude, s.cfg.Longitude)
	quotesSvc := service.NewQuotesService(hc, s.cfg.APINinjasAPIKey, cache)
	notificationRepo := repository.NewSQLiteNotificationRepository(s.db)
	notificationSvc := service.NewNotificationService(notificationRepo)
	watchedRepoRepo := repository.NewSQLiteWatchedRepoRepository(s.db)
	integrationRepo := repository.NewSQLiteIntegrationRepository(s.db)
	webhookSvc := service.NewWebhookService(watchedRepoRepo, notificationRepo, s.encSvc)
	githubIntegrationSvc := service.NewGitHubIntegrationService(
		integrationRepo, watchedRepoRepo, s.encSvc, hc,
		s.cfg.GitHubClientID, s.cfg.GitHubClientSecret, s.cfg.GitHubCallbackURL, s.cfg.GitHubWebhookURL,
	)

	// Social feed
	publisher := events.Publisher(&events.NoopPublisher{})
	postsRepo := repository.NewSQLitePostsRepository(s.db)
	postsSvc := service.NewPostsService(postsRepo, publisher)
	followRepo := repository.NewSQLiteFollowRepository(s.db)
	followSvc := service.NewFollowService(followRepo, publisher)
	feedRepo := repository.NewSQLiteFeedRepository(s.db)
	feedSvc := service.NewFeedService(feedRepo)

	// Auth
	authSvc := service.NewAuthService(s.db, s.cfg.GoogleClientID, s.cfg.GoogleClientSecret, s.cfg.GoogleCallbackURL)
	authH := handler.NewAuthHandler(authSvc, s.cfg.SessionKey, s.cfg.FrontendURL, s.cfg.SecureCookies)
	requireAuth := middleware.RequireAuth(s.cfg.SessionKey)
	meH := handler.NewMeHandler()

	// Handlers
	weatherH := handler.NewWeatherHandler(weatherSvc)
	newsH := handler.NewNewsHandler(newsSvc)
	stocksH := handler.NewStocksHandler(stocksSvc, v)
	calendarH := handler.NewCalendarHandler(calendarSvc)
	tasksH := handler.NewTasksHandler(tasksSvc, v)
	settingsH := handler.NewUserSettingsHandler(settingsSvc, v)
	labelsH := handler.NewTaskLabelsHandler(labelsSvc, v)
	metaH := handler.NewMetaHandler(sunriseSvc, quotesSvc)
	billsH := handler.NewBillsHandler(billsSvc, v)
	notificationsH := handler.NewNotificationsHandler(notificationSvc, v)
	webhooksH := handler.NewWebhooksHandler(webhookSvc, v, s.cfg.AppEnv)
	githubAuthH := handler.NewGitHubAuthHandler(githubIntegrationSvc, s.cfg.FrontendURL, s.cfg.SecureCookies)
	integrationsH := handler.NewIntegrationsHandler(githubIntegrationSvc, v)
	postsH := handler.NewPostsHandler(postsSvc, v)
	followH := handler.NewFollowHandler(followSvc, v)
	feedH := handler.NewFeedHandler(feedSvc)
	dashboardH := handler.NewDashboardHandler(
		weatherSvc,
		stocksSvc,
		calendarSvc,
		tasksSvc,
		sunriseSvc,
		quotesSvc,
		billsSvc,
		notificationSvc,
	)

	// Public routes — no session required
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		response.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	authH.AddRoutes(r)
	webhooksH.AddRoutes(r)

	// Protected routes — valid session cookie required
	r.Group(func(r chi.Router) {
		r.Use(requireAuth)

		meH.AddRoutes(r)
		dashboardH.AddRoutes(r)
		weatherH.AddRoutes(r)
		newsH.AddRoutes(r)
		stocksH.AddRoutes(r)
		calendarH.AddRoutes(r)
		metaH.AddRoutes(r)
		tasksH.AddRoutes(r)
		settingsH.AddRoutes(r)
		labelsH.AddRoutes(r)
		billsH.AddRoutes(r)
		notificationsH.AddRoutes(r)
		githubAuthH.AddRoutes(r)
		integrationsH.AddRoutes(r)
		postsH.AddRoutes(r)
		followH.AddRoutes(r)
		feedH.AddRoutes(r)
	})
}
