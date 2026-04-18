package service

import (
	"context"
	"fmt"
	"time"

	platformcache "github.com/meowmix1337/argus/backend/internal/platform/cache"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

// SunriseService fetches sunrise/sunset data from sunrise-sunset.org.
type SunriseService struct {
	httpClient httpclient.HTTPClient
	cache      *platformcache.CacheService
	lat        float64
	lon        float64
}

// NewSunriseService creates a new SunriseService.
func NewSunriseService(httpClient httpclient.HTTPClient, cache *platformcache.CacheService, lat, lon float64) *SunriseService {
	return &SunriseService{
		httpClient: httpClient,
		cache:      cache,
		lat:        lat,
		lon:        lon,
	}
}

const sunriseCacheTTL = 6 * time.Hour

// Fetch retrieves sunrise and sunset times.
func (s *SunriseService) Fetch(ctx context.Context) (string, string, string, error) {
	const cacheKey = "sunrise"
	if v, ok := s.cache.Get(cacheKey); ok {
		d := v.(sunriseResult)
		return d.Sunrise, d.Sunset, d.Daylight, nil
	}

	sunrise, sunset, daylight, err := s.fetchFromAPI(ctx)
	if err != nil {
		return "", "", "", err
	}

	s.cache.Set(cacheKey, sunriseResult{sunrise, sunset, daylight}, sunriseCacheTTL)
	return sunrise, sunset, daylight, nil
}

type sunriseResult struct {
	Sunrise  string
	Sunset   string
	Daylight string
}

func (s *SunriseService) fetchFromAPI(ctx context.Context) (string, string, string, error) {
	u := fmt.Sprintf(
		"https://api.sunrise-sunset.org/json?lat=%f&lng=%f&formatted=0",
		s.lat, s.lon,
	)

	var apiResp sunriseSunsetResponse
	if err := s.httpClient.Get(ctx, u, &apiResp); err != nil {
		return "", "", "", err
	}

	if apiResp.Status != "OK" {
		return "", "", "", fmt.Errorf("sunrise API returned status: %s", apiResp.Status)
	}

	local := time.Local

	sunriseTime, err := time.Parse(time.RFC3339, apiResp.Results.Sunrise)
	if err != nil {
		return "", "", "", err
	}
	sunsetTime, err := time.Parse(time.RFC3339, apiResp.Results.Sunset)
	if err != nil {
		return "", "", "", err
	}

	daylight := sunsetTime.Sub(sunriseTime)
	hours := int(daylight.Hours())
	minutes := int(daylight.Minutes()) % 60

	return sunriseTime.In(local).Format("3:04 PM"),
		sunsetTime.In(local).Format("3:04 PM"),
		fmt.Sprintf("%dh %dm", hours, minutes),
		nil
}

type sunriseSunsetResponse struct {
	Results struct {
		Sunrise string `json:"sunrise"`
		Sunset  string `json:"sunset"`
	} `json:"results"`
	Status string `json:"status"`
}
