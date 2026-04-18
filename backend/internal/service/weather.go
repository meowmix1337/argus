package service

import (
	"context"
	"fmt"
	"time"

	"github.com/meowmix1337/argus/backend/internal/model"
	"github.com/meowmix1337/argus/backend/internal/platform/httpclient"
)

// WMO weather code → (condition, emoji icon)
var wmoConditions = map[int][2]string{
	0:  {"Clear Sky", "☀️"},
	1:  {"Mainly Clear", "🌤️"},
	2:  {"Partly Cloudy", "⛅"},
	3:  {"Overcast", "☁️"},
	45: {"Foggy", "🌫️"},
	48: {"Icy Fog", "🌫️"},
	51: {"Light Drizzle", "🌦️"},
	53: {"Drizzle", "🌦️"},
	55: {"Dense Drizzle", "🌦️"},
	61: {"Slight Rain", "🌧️"},
	63: {"Rain", "🌧️"},
	65: {"Heavy Rain", "🌧️"},
	71: {"Slight Snow", "🌨️"},
	73: {"Snow", "❄️"},
	75: {"Heavy Snow", "❄️"},
	77: {"Snow Grains", "🌨️"},
	80: {"Rain Showers", "🌦️"},
	81: {"Rain Showers", "🌦️"},
	82: {"Violent Rain", "⛈️"},
	85: {"Snow Showers", "🌨️"},
	86: {"Heavy Snow Showers", "❄️"},
	95: {"Thunderstorm", "⛈️"},
	96: {"Thunderstorm w/ Hail", "⛈️"},
	99: {"Thunderstorm w/ Hail", "⛈️"},
}

func wmoToCondition(code int) (string, string) {
	if c, ok := wmoConditions[code]; ok {
		return c[0], c[1]
	}
	return "Unknown", "🌡️"
}

// WeatherService fetches weather data from Open-Meteo.
type WeatherService struct {
	httpClient httpclient.HTTPClient
	cache      *CacheService
	lat        float64
	lon        float64
}

// NewWeatherService creates a new WeatherService.
func NewWeatherService(httpClient httpclient.HTTPClient, cache *CacheService, lat, lon float64) *WeatherService {
	return &WeatherService{
		httpClient: httpClient,
		cache:      cache,
		lat:        lat,
		lon:        lon,
	}
}

const weatherCacheTTL = 15 * time.Minute

// Fetch retrieves current weather, hourly forecast, and air quality data.
func (s *WeatherService) Fetch(ctx context.Context) (model.WeatherData, error) {
	const cacheKey = "weather"
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.(model.WeatherData), nil
	}

	data, err := s.fetchFromAPI(ctx)
	if err != nil {
		return model.WeatherData{}, err
	}

	s.cache.Set(cacheKey, data, weatherCacheTTL)
	return data, nil
}

func (s *WeatherService) fetchFromAPI(ctx context.Context) (model.WeatherData, error) {
	// Fetch current + hourly + daily
	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
			"&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m"+
			"&hourly=temperature_2m,weather_code,uv_index&daily=temperature_2m_max,temperature_2m_min"+
			"&temperature_unit=fahrenheit&wind_speed_unit=mph&timezone=auto&forecast_days=1",
		s.lat, s.lon,
	)

	var forecast openMeteoForecast
	if err := s.httpClient.Get(ctx, forecastURL, &forecast); err != nil {
		return model.WeatherData{}, err
	}

	// Fetch air quality
	aqiURL := fmt.Sprintf(
		"https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%f&longitude=%f&current=us_aqi",
		s.lat, s.lon,
	)
	var aqiResp openMeteoAQI
	_ = s.httpClient.Get(ctx, aqiURL, &aqiResp) // AQI is optional

	condition, icon := wmoToCondition(forecast.Current.WeatherCode)

	// Build hourly strip (next 8 hours).
	// API times are in local timezone (timezone=auto), so compare as strings
	// to avoid UTC/local mismatch from time.Parse.
	hourly := make([]model.HourlyForecast, 0, 8)
	nowStr := time.Now().Format("2006-01-02T15:04")
	uvIndex := 0.0
	count := 0
	for i, t := range forecast.Hourly.Time {
		if count >= 8 {
			break
		}
		if t < nowStr {
			continue
		}
		// Use UV index from the first upcoming hourly slot as the "current" value
		if count == 0 && i < len(forecast.Hourly.UVIndex) {
			uvIndex = forecast.Hourly.UVIndex[i]
		}
		label := "Now"
		if count > 0 {
			// Parse just to get a nice label; ignore timezone
			if parsed, err := time.Parse("2006-01-02T15:04", t); err == nil {
				label = parsed.Format("3pm")
			} else {
				label = t[11:16] // fallback: "HH:MM"
			}
		}
		_, hIcon := wmoToCondition(forecast.Hourly.WeatherCode[i])
		hourly = append(hourly, model.HourlyForecast{
			Time: label,
			Temp: forecast.Hourly.Temperature2m[i],
			Icon: hIcon,
		})
		count++
	}

	aqi := aqiResp.Current.USAQI
	aqiLabel := aqiCategory(aqi)

	high := 0.0
	low := 0.0
	if len(forecast.Daily.Temperature2mMax) > 0 {
		high = forecast.Daily.Temperature2mMax[0]
	}
	if len(forecast.Daily.Temperature2mMin) > 0 {
		low = forecast.Daily.Temperature2mMin[0]
	}

	return model.WeatherData{
		Temp:      forecast.Current.Temperature2m,
		High:      high,
		Low:       low,
		Condition: condition,
		Icon:      icon,
		Humidity:  forecast.Current.RelativeHumidity2m,
		WindSpeed: forecast.Current.WindSpeed10m,
		UVIndex:   uvIndex,
		AQI:       aqi,
		AQILabel:  aqiLabel,
		Hourly:    hourly,
	}, nil
}

func aqiCategory(aqi int) string {
	switch {
	case aqi <= 50:
		return "Good"
	case aqi <= 100:
		return "Moderate"
	case aqi <= 150:
		return "Unhealthy for Sensitive"
	case aqi <= 200:
		return "Unhealthy"
	case aqi <= 300:
		return "Very Unhealthy"
	default:
		return "Hazardous"
	}
}

// Open-Meteo API response types
type openMeteoForecast struct {
	Current struct {
		Temperature2m      float64 `json:"temperature_2m"`
		RelativeHumidity2m int     `json:"relative_humidity_2m"`
		WeatherCode        int     `json:"weather_code"`
		WindSpeed10m       float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Hourly struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
		WeatherCode   []int     `json:"weather_code"`
		UVIndex       []float64 `json:"uv_index"`
	} `json:"hourly"`
	Daily struct {
		Temperature2mMax []float64 `json:"temperature_2m_max"`
		Temperature2mMin []float64 `json:"temperature_2m_min"`
	} `json:"daily"`
}

type openMeteoAQI struct {
	Current struct {
		USAQI int `json:"us_aqi"`
	} `json:"current"`
}
