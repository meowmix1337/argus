package service

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSunriseService_Fetch_CacheHit(t *testing.T) {
	cache := NewCacheService()
	// Pre-populate cache; if the HTTP client is called the test fails.
	cache.Set("sunrise", sunriseResult{"6:00 AM", "7:30 PM", "13h 30m"}, time.Minute)

	svc := NewSunriseService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP client must not be called on cache hit")},
		cache, 37.77, -122.41,
	)

	sunrise, sunset, daylight, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if sunrise != "6:00 AM" || sunset != "7:30 PM" || daylight != "13h 30m" {
		t.Errorf("cache hit returned wrong values: %q %q %q", sunrise, sunset, daylight)
	}
}

func TestSunriseService_Fetch_HTTPError(t *testing.T) {
	svc := NewSunriseService(
		&fakeHTTPClient{err: fmt.Errorf("network failure")},
		NewCacheService(), 37.77, -122.41,
	)

	_, _, _, err := svc.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error on HTTP failure")
	}
}

func TestSunriseService_Fetch_StatusNotOK(t *testing.T) {
	// API returns a non-OK status (e.g. invalid coordinates).
	resp := sunriseSunsetResponse{Status: "INVALID_REQUEST"}
	svc := NewSunriseService(
		&fakeHTTPClient{responseBody: resp},
		NewCacheService(), 37.77, -122.41,
	)

	_, _, _, err := svc.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected error when API status is not 'OK'")
	}
}

func TestSunriseService_Fetch_ComputesDaylightDuration(t *testing.T) {
	// 6 AM → 8 PM = 14h 0m daylight
	resp := sunriseSunsetResponse{
		Results: struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		}{
			Sunrise: "2024-06-01T06:00:00+00:00",
			Sunset:  "2024-06-01T20:00:00+00:00",
		},
		Status: "OK",
	}
	svc := NewSunriseService(&fakeHTTPClient{responseBody: resp}, NewCacheService(), 37.77, -122.41)

	_, _, daylight, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if daylight != "14h 0m" {
		t.Errorf("daylight = %q, want %q", daylight, "14h 0m")
	}
}

// TestSunriseService_Fetch_BadSunriseTime covers the time.Parse error path for Sunrise.
func TestSunriseService_Fetch_BadSunriseTime(t *testing.T) {
	resp := sunriseSunsetResponse{
		Results: struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		}{
			Sunrise: "not-a-time",
			Sunset:  "2024-06-01T20:00:00+00:00",
		},
		Status: "OK",
	}
	svc := NewSunriseService(&fakeHTTPClient{responseBody: resp}, NewCacheService(), 37.77, -122.41)
	if _, _, _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error for unparseable sunrise time, got nil")
	}
}

// TestSunriseService_Fetch_BadSunsetTime covers the time.Parse error path for Sunset.
func TestSunriseService_Fetch_BadSunsetTime(t *testing.T) {
	resp := sunriseSunsetResponse{
		Results: struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		}{
			Sunrise: "2024-06-01T06:00:00+00:00",
			Sunset:  "not-a-time",
		},
		Status: "OK",
	}
	svc := NewSunriseService(&fakeHTTPClient{responseBody: resp}, NewCacheService(), 37.77, -122.41)
	if _, _, _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error for unparseable sunset time, got nil")
	}
}

func TestSunriseService_Fetch_PopulatesCache(t *testing.T) {
	resp := sunriseSunsetResponse{
		Results: struct {
			Sunrise string `json:"sunrise"`
			Sunset  string `json:"sunset"`
		}{
			Sunrise: "2024-06-01T06:30:00+00:00",
			Sunset:  "2024-06-01T20:00:00+00:00",
		},
		Status: "OK",
	}
	cache := NewCacheService()
	svc := NewSunriseService(&fakeHTTPClient{responseBody: resp}, cache, 37.77, -122.41)

	if _, _, _, err := svc.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, ok := cache.Get("sunrise"); !ok {
		t.Error("expected result to be cached after a successful fetch")
	}
}
