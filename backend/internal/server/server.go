package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"

	calendarhandler "github.com/meowmix1337/argus/backend/internal/domain/external/calendar/handler"
	calendarsvc "github.com/meowmix1337/argus/backend/internal/domain/external/calendar/service"
	metahandler "github.com/meowmix1337/argus/backend/internal/domain/external/meta/handler"
	newshandler "github.com/meowmix1337/argus/backend/internal/domain/external/news/handler"
	newssvc "github.com/meowmix1337/argus/backend/internal/domain/external/news/service"
	quotessvc "github.com/meowmix1337/argus/backend/internal/domain/external/quotes/service"
	sunrisesvc "github.com/meowmix1337/argus/backend/internal/domain/external/sunrise/service"
	weatherhandler "github.com/meowmix1337/argus/backend/internal/domain/external/weather/handler"
	weathersvc "github.com/meowmix1337/argus/backend/internal/domain/external/weather/service"
	financehandler "github.com/meowmix1337/argus/backend/internal/domain/finance/handler"
	financerepo "github.com/meowmix1337/argus/backend/internal/domain/finance/repository"
	financesvc "github.com/meowmix1337/argus/backend/internal/domain/finance/service"
	integrationshandler "github.com/meowmix1337/argus/backend/internal/domain/integrations/handler"
	integrationsrepo "github.com/meowmix1337/argus/backend/internal/domain/integrations/repository"
	integrationssvc "github.com/meowmix1337/argus/backend/internal/domain/integrations/service"
	notificationshandler "github.com/meowmix1337/argus/backend/internal/domain/notifications/handler"
	notificationsrepo "github.com/meowmix1337/argus/backend/internal/domain/notifications/repository"
	notificationssvc "github.com/meowmix1337/argus/backend/internal/domain/notifications/service"
	socialconsumer "github.com/meowmix1337/argus/backend/internal/domain/social/consumer"
	socialhandler "github.com/meowmix1337/argus/backend/internal/domain/social/handler"
	socialrepo "github.com/meowmix1337/argus/backend/internal/domain/social/repository"
	socialsvc "github.com/meowmix1337/argus/backend/internal/domain/social/service"
	taskshandler "github.com/meowmix1337/argus/backend/internal/domain/tasks/handler"
	tasksrepo "github.com/meowmix1337/argus/backend/internal/domain/tasks/repository"
	taskssvc "github.com/meowmix1337/argus/backend/internal/domain/tasks/service"
	usershandler "github.com/meowmix1337/argus/backend/internal/domain/users/handler"
	usersrepo "github.com/meowmix1337/argus/backend/internal/domain/users/repository"
	userssvc "github.com/meowmix1337/argus/backend/internal/domain/users/service"
	"github.com/meowmix1337/argus/backend/internal/events"
	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
	"github.com/meowmix1337/argus/backend/internal/platform/config"
	platformcrypto "github.com/meowmix1337/argus/backend/internal/platform/crypto"
	"github.com/meowmix1337/argus/backend/internal/platform/eventbus"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/publisher"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/platform/validate"
)

// Server holds the HTTP router and all dependencies.
type Server struct {
	router    *chi.Mux
	cfg       *config.Config
	db        *sqlx.DB
	encSvc    *platformcrypto.EncryptionService // nil means no encryption
	publisher publisher.Publisher
	cm        *eventbus.ConsumerManager // nil when NSQ is not configured
}

// New creates a new Server with all services, handlers, and routes registered.
func New(cfg *config.Config, db *sqlx.DB, encSvc *platformcrypto.EncryptionService) *Server {
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

// Stop gracefully shuts down background workers (NSQ consumers, publisher).
func (s *Server) Stop() {
	if s.cm != nil {
		s.cm.Stop()
	}
	if s.publisher != nil {
		s.publisher.Stop()
	}
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
	cache := platformcache.NewCacheService()
	v := validate.New()

	// Services
	weatherSvc := weathersvc.NewWeatherService(hc, cache, s.cfg.Latitude, s.cfg.Longitude)
	newsSvc := newssvc.NewNewsService(hc, s.cfg.GNewsAPIKey, cache)
	watchlistRepo := financerepo.NewSQLiteStocksWatchlistRepository(s.db)
	stocksSvc := financesvc.NewStocksService(hc, s.cfg.FinnhubAPIKey, cache, watchlistRepo)
	taskRepo := tasksrepo.NewSQLiteTaskRepository(s.db)
	tasksSvc := taskssvc.NewTasksService(taskRepo)
	billsRepo := financerepo.NewSQLiteBillsRepository(s.db)
	billPaymentsRepo := financerepo.NewSQLiteBillPaymentsRepository(s.db)
	billsSvc := financesvc.NewBillsService(billsRepo, billPaymentsRepo)
	settingsRepo := usersrepo.NewSQLiteUserSettingsRepository(s.db)
	settingsSvc := userssvc.NewUserSettingsService(settingsRepo, s.encSvc)
	calendarSvc := calendarsvc.NewCalendarService(hc, cache, s.cfg.Timezone, settingsSvc)
	labelRepo := tasksrepo.NewSQLiteTaskLabelsRepository(s.db)
	labelsSvc := taskssvc.NewTaskLabelsService(labelRepo)
	sunriseSvc := sunrisesvc.NewSunriseService(hc, cache, s.cfg.Latitude, s.cfg.Longitude)
	quotesSvc := quotessvc.NewQuotesService(hc, s.cfg.APINinjasAPIKey, cache)
	notificationRepo := notificationsrepo.NewSQLiteNotificationRepository(s.db)
	notificationSvc := notificationssvc.NewNotificationService(notificationRepo)
	socialPrefsRepo := socialrepo.NewSQLiteSocialPrefsRepository(s.db)
	socialPrefsSvc := socialsvc.NewSocialPrefsService(socialPrefsRepo)
	watchedRepoRepo := integrationsrepo.NewSQLiteWatchedRepoRepository(s.db)
	integrationRepo := integrationsrepo.NewSQLiteIntegrationRepository(s.db)
	webhookSvc := integrationssvc.NewWebhookService(watchedRepoRepo, notificationRepo, s.encSvc)
	githubIntegrationSvc := integrationssvc.NewGitHubIntegrationService(
		integrationRepo, watchedRepoRepo, s.encSvc, hc,
		s.cfg.GitHubClientID, s.cfg.GitHubClientSecret, s.cfg.GitHubCallbackURL, s.cfg.GitHubWebhookURL,
	)

	// Social feed — publisher uses real NSQ when NSQD_ADDR is set, otherwise noop.
	s.publisher = buildPublisher(s.cfg.NSQDAddr)
	postsRepo := socialrepo.NewSQLitePostsRepository(s.db)
	postsSvc := socialsvc.NewPostsService(postsRepo, s.publisher)
	followRepo := socialrepo.NewSQLiteFollowRepository(s.db)
	followSvc := socialsvc.NewFollowService(followRepo, s.publisher)
	feedRepo := socialrepo.NewSQLiteFeedRepository(s.db)
	feedSvc := socialsvc.NewFeedService(feedRepo)
	usersRepo := usersrepo.NewSQLiteUsersRepository(s.db)
	usersSvc := userssvc.NewUserService(usersRepo)

	// NSQ consumers — only started when NSQ_LOOKUPD_ADDR is configured.
	if s.cfg.NSQLookupdAddr != "" {
		cm := eventbus.NewConsumerManager(s.cfg.NSQLookupdAddr)
		for _, consumer := range []eventbus.MessageHandler{
			socialconsumer.NewFeedFanoutConsumer(followRepo, feedRepo),
			socialconsumer.NewFollowBackfillConsumer(postsRepo, feedRepo),
			events.NewFollowerNotificationConsumer(followRepo, notificationSvc, socialPrefsSvc),
			events.NewFollowNotificationConsumer(notificationSvc, socialPrefsSvc),
		} {
			if err := cm.Register(consumer); err != nil {
				slog.Warn("failed to register consumer", "topic", consumer.Topic(), "error", err)
			}
		}
		if err := cm.Start(); err != nil {
			slog.Warn("failed to start consumer manager", "error", err)
			cm.Stop()
		} else {
			s.cm = cm
		}
	}

	// Auth
	authSvc := userssvc.NewAuthService(s.db, s.cfg.GoogleClientID, s.cfg.GoogleClientSecret, s.cfg.GoogleCallbackURL)
	authH := usershandler.NewAuthHandler(authSvc, s.cfg.SessionKey, s.cfg.FrontendURL, s.cfg.SecureCookies)
	requireAuth := middleware.RequireAuth(s.cfg.SessionKey)
	meH := usershandler.NewMeHandler()

	// Handlers
	weatherH := weatherhandler.NewWeatherHandler(weatherSvc)
	newsH := newshandler.NewNewsHandler(newsSvc)
	stocksH := financehandler.NewStocksHandler(stocksSvc, v)
	calendarH := calendarhandler.NewCalendarHandler(calendarSvc)
	tasksH := taskshandler.NewTasksHandler(tasksSvc, v)
	settingsH := usershandler.NewUserSettingsHandler(settingsSvc, v)
	labelsH := taskshandler.NewTaskLabelsHandler(labelsSvc, v)
	metaH := metahandler.NewMetaHandler(sunriseSvc, quotesSvc)
	billsH := financehandler.NewBillsHandler(billsSvc, v)
	notificationsH := notificationshandler.NewNotificationsHandler(notificationSvc, v)
	socialPrefsH := socialhandler.NewSocialPrefsHandler(socialPrefsSvc, v)
	webhooksH := integrationshandler.NewWebhooksHandler(webhookSvc, v, s.cfg.AppEnv)
	githubAuthH := integrationshandler.NewGitHubAuthHandler(githubIntegrationSvc, s.cfg.FrontendURL, s.cfg.SecureCookies)
	integrationsH := integrationshandler.NewIntegrationsHandler(githubIntegrationSvc, v)
	postsH := socialhandler.NewPostsHandler(postsSvc, v)
	followH := socialhandler.NewFollowHandler(followSvc, v)
	feedH := socialhandler.NewFeedHandler(feedSvc)
	usersH := usershandler.NewUsersHandler(usersSvc)
	dashboardH := NewDashboardHandler(
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
		socialPrefsH.AddRoutes(r)
		githubAuthH.AddRoutes(r)
		integrationsH.AddRoutes(r)
		postsH.AddRoutes(r)
		followH.AddRoutes(r)
		feedH.AddRoutes(r)
		usersH.AddRoutes(r)
	})
}

// buildPublisher returns a real NSQ publisher when nsqdAddr is set, or a noop publisher otherwise.
func buildPublisher(nsqdAddr string) publisher.Publisher {
	if nsqdAddr == "" {
		return &publisher.NoopPublisher{}
	}
	p, err := publisher.NewNSQPublisher(nsqdAddr)
	if err != nil {
		slog.Warn("failed to create NSQ publisher, falling back to noop", "error", err)
		return &publisher.NoopPublisher{}
	}
	return p
}
