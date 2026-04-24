package handler

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	financesvc "github.com/meowmix1337/argus/backend/internal/domain/finance/service"
	notificationssvc "github.com/meowmix1337/argus/backend/internal/domain/notifications/service"
	taskssvc "github.com/meowmix1337/argus/backend/internal/domain/tasks/service"
	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/middleware"
	"github.com/meowmix1337/argus/backend/internal/platform/response"
	"github.com/meowmix1337/argus/backend/internal/service"
)

const defaultTaskLimit = 5

// DashboardHandler aggregates all widget data into a single response.
type DashboardHandler struct {
	weather       *service.WeatherService
	stocks        *financesvc.StocksService
	calendar      *service.CalendarService
	tasks         *taskssvc.TasksService
	sunrise       *service.SunriseService
	quotes        *service.QuotesService
	bills         *financesvc.BillsService
	notifications *notificationssvc.NotificationService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(
	weather *service.WeatherService,
	stocks *financesvc.StocksService,
	calendar *service.CalendarService,
	tasks *taskssvc.TasksService,
	sunrise *service.SunriseService,
	quotes *service.QuotesService,
	bills *financesvc.BillsService,
	notifications *notificationssvc.NotificationService,
) *DashboardHandler {
	return &DashboardHandler{
		weather:       weather,
		stocks:        stocks,
		calendar:      calendar,
		tasks:         tasks,
		sunrise:       sunrise,
		quotes:        quotes,
		bills:         bills,
		notifications: notifications,
	}
}

func (h *DashboardHandler) AddRoutes(r chi.Router) {
	r.Get("/api/dashboard", h.Get)
}

// Get fans out to all services concurrently and returns a unified response.
// If any individual service fails, that field is left nil/empty.
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := middleware.UserIDFromRequest(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	// Each goroutine below writes to a distinct field of resp (Weather, Stocks,
	// Calendar, Tasks, Meta). g.Wait() provides the happens-before guarantee
	// needed for the final read of resp after all goroutines complete, so no
	// mutex is required.
	var resp model.DashboardResponse

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		data, err := h.weather.Fetch(gctx)
		if err != nil {
			slog.Warn("weather unavailable", "error", err)
			return nil
		}
		resp.Weather = &data
		return nil
	})

	g.Go(func() error {
		sess, ok := middleware.SessionFromContext(gctx)
		if !ok {
			slog.Warn("stocks fetch skipped: no session in context")
			return nil
		}
		data, err := h.stocks.Fetch(gctx, sess.UserID)
		if err != nil {
			slog.Warn("stocks unavailable", "error", err)
			return nil
		}
		resp.Stocks = data
		return nil
	})

	g.Go(func() error {
		data, err := h.calendar.Fetch(gctx, userID)
		if err != nil {
			slog.Warn("calendar fetch failed", "error", err)
			return nil
		}
		resp.Calendar = data
		return nil
	})

	g.Go(func() error {
		sess, ok := middleware.SessionFromContext(gctx)
		if !ok {
			slog.Warn("tasks fetch skipped: no session in context")
			return nil
		}
		data, total, err := h.tasks.List(gctx, sess.UserID, defaultTaskLimit, 0)
		if err != nil {
			slog.Warn("tasks fetch failed", "error", err)
			return nil
		}
		resp.Tasks = data
		resp.TasksTotal = total
		return nil
	})

	g.Go(func() error {
		sunriseTime, sunsetTime, daylight, err := h.sunrise.Fetch(gctx)
		if err != nil {
			slog.Warn("sunrise unavailable", "error", err)
		}
		quote, err := h.quotes.Fetch(gctx)
		if err != nil {
			slog.Warn("quotes unavailable", "error", err)
		}
		meta := model.MetaData{
			Sunrise:  sunriseTime,
			Sunset:   sunsetTime,
			Daylight: daylight,
			Quote:    quote,
		}
		resp.Meta = &meta
		return nil
	})

	g.Go(func() error {
		sess, ok := middleware.SessionFromContext(gctx)
		if !ok {
			slog.Warn("bills fetch skipped: no session in context")
			return nil
		}
		data, err := h.bills.ListCurrentMonth(gctx, sess.UserID)
		if err != nil {
			slog.Warn("bills fetch failed", "error", err)
			return nil
		}
		resp.Bills = data
		return nil
	})

	g.Go(func() error {
		sess, ok := middleware.SessionFromContext(gctx)
		if !ok {
			slog.Warn("unread notifications fetch skipped: no session in context")
			return nil
		}
		count, err := h.notifications.CountUnread(gctx, sess.UserID)
		if err != nil {
			slog.Warn("unread notifications count failed", "error", err)
			return nil
		}
		resp.UnreadNotifications = count
		return nil
	})

	// errgroup.Wait() will not return a non-nil error since we always return nil
	_ = g.Wait()

	response.WriteJSON(w, http.StatusOK, resp)
}
