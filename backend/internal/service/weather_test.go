package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
)

func TestWmoToCondition_KnownCodes(t *testing.T) {
	cases := []struct {
		code      int
		condition string
		icon      string
	}{
		{0, "Clear Sky", "☀️"},
		{1, "Mainly Clear", "🌤️"},
		{2, "Partly Cloudy", "⛅"},
		{3, "Overcast", "☁️"},
		{45, "Foggy", "🌫️"},
		{48, "Icy Fog", "🌫️"},
		{51, "Light Drizzle", "🌦️"},
		{63, "Rain", "🌧️"},
		{73, "Snow", "❄️"},
		{75, "Heavy Snow", "❄️"},
		{82, "Violent Rain", "⛈️"},
		{95, "Thunderstorm", "⛈️"},
		{99, "Thunderstorm w/ Hail", "⛈️"},
	}
	for _, tc := range cases {
		cond, icon := wmoToCondition(tc.code)
		if cond != tc.condition {
			t.Errorf("wmoToCondition(%d).condition = %q, want %q", tc.code, cond, tc.condition)
		}
		if icon != tc.icon {
			t.Errorf("wmoToCondition(%d).icon = %q, want %q", tc.code, icon, tc.icon)
		}
	}
}

func TestWmoToCondition_UnknownCode(t *testing.T) {
	cond, icon := wmoToCondition(999)
	if cond != "Unknown" {
		t.Errorf("expected Unknown, got %q", cond)
	}
	if icon != "🌡️" {
		t.Errorf("expected 🌡️, got %q", icon)
	}
}

// ---- WeatherService.Fetch ----

func TestWeatherService_Fetch_CacheHit(t *testing.T) {
	cache := platformcache.NewCacheService()
	cached := model.WeatherData{Temp: 72.0, Condition: "Clear Sky"}
	cache.Set("weather", cached, time.Minute)

	svc := NewWeatherService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP must not be called on cache hit")},
		cache, 37.77, -122.41,
	)
	data, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if data.Condition != "Clear Sky" {
		t.Errorf("Condition = %q, want cached %q", data.Condition, "Clear Sky")
	}
}

func TestWeatherService_Fetch_Success(t *testing.T) {
	forecast := openMeteoForecast{}
	forecast.Current.Temperature2m = 68.0
	forecast.Current.WeatherCode = 1 // Mainly Clear
	forecast.Current.RelativeHumidity2m = 55
	forecast.Current.WindSpeed10m = 10.0
	forecast.Daily.Temperature2mMax = []float64{75.0}
	forecast.Daily.Temperature2mMin = []float64{55.0}

	cache := platformcache.NewCacheService()
	svc := NewWeatherService(&fakeHTTPClient{responseBody: forecast}, cache, 37.77, -122.41)

	data, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if data.Temp != 68.0 {
		t.Errorf("Temp = %v, want 68.0", data.Temp)
	}
	if data.High != 75.0 {
		t.Errorf("High = %v, want 75.0", data.High)
	}
	if data.Condition != "Mainly Clear" {
		t.Errorf("Condition = %q, want %q", data.Condition, "Mainly Clear")
	}
}

func TestWeatherService_Fetch_PopulatesCache(t *testing.T) {
	forecast := openMeteoForecast{}
	forecast.Current.Temperature2m = 65.0

	cache := platformcache.NewCacheService()
	svc := NewWeatherService(&fakeHTTPClient{responseBody: forecast}, cache, 37.77, -122.41)

	if _, err := svc.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, ok := cache.Get("weather"); !ok {
		t.Error("expected result to be cached after successful fetch")
	}
}

func TestWeatherService_Fetch_HTTPError_Propagates(t *testing.T) {
	cache := platformcache.NewCacheService()
	svc := NewWeatherService(&fakeHTTPClient{err: fmt.Errorf("network failure")}, cache, 37.77, -122.41)
	if _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error on HTTP failure")
	}
}

func TestAqiCategory(t *testing.T) {
	cases := []struct {
		aqi  int
		want string
	}{
		{0, "Good"},
		{50, "Good"},
		{51, "Moderate"},
		{100, "Moderate"},
		{101, "Unhealthy for Sensitive"},
		{150, "Unhealthy for Sensitive"},
		{151, "Unhealthy"},
		{200, "Unhealthy"},
		{201, "Very Unhealthy"},
		{300, "Very Unhealthy"},
		{301, "Hazardous"},
		{999, "Hazardous"},
	}
	for _, tc := range cases {
		if got := aqiCategory(tc.aqi); got != tc.want {
			t.Errorf("aqiCategory(%d) = %q, want %q", tc.aqi, got, tc.want)
		}
	}
}
