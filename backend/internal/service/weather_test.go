package service

import "testing"

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
