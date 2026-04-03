export interface HourlyForecast {
  time: string;
  temp: number;
  icon: string;
}

export interface WeatherData {
  temp: number;
  high: number;
  low: number;
  condition: string;
  icon: string;
  humidity: number;
  windSpeed: number;
  uvIndex: number;
  aqi: number;
  aqiLabel: string;
  hourly: HourlyForecast[];
}

export interface CalendarEvent {
  time: string;
  title: string;
  color: string;
  duration: string;
}

export interface Task {
  id: string;
  text: string;
  done: boolean;
  priority: 'high' | 'medium' | 'low';
}

export interface NewsItem {
  title: string;
  source: string;
  time: string;
  url: string;
}

export interface NewsCategory {
  name: string;
  items: NewsItem[];
}

export interface StockQuote {
  symbol: string;
  price: number;
  change: number;
  pct: number;
}

export interface SymbolSearchResult {
  symbol: string;
  description: string;
  type: string;
}

export interface Quote {
  text: string;
  author: string;
}

export interface MetaData {
  sunrise: string;
  sunset: string;
  daylight: string;
  quote: Quote;
}

export interface Bill {
  id: string;
  name: string;
  amount: number | null;
  categoryId: string;
  recurrenceType: 'once' | 'weekly' | 'biweekly' | 'monthly' | 'quarterly' | 'annual';
  dueDate: string | null;    // YYYY-MM-DD; for 'once'
  dueDay: number | null;     // 1-31; for 'monthly' and 'annual'
  dueMonth: number | null;   // 1-12; for 'annual'
  anchorDate: string | null; // YYYY-MM-DD; for 'weekly','biweekly','quarterly'
  notes: string | null;
  createdAt: string;
}

export interface BillDue {
  id: string;
  name: string;
  amount: number | null;
  categoryId: string;
  recurrenceType: string;
  notes: string | null;
  computedDueDate: string; // YYYY-MM-DD
  isPaid: boolean;
  paidDate: string | null;  // YYYY-MM-DD
  paidNote: string | null;
  paymentId: string | null; // used for DELETE (unmark)
}

export interface DashboardResponse {
  weather: WeatherData | null;
  calendar: CalendarEvent[];
  tasks: Task[];
  tasksTotal?: number;
  stocks: StockQuote[] | null;
  meta: MetaData | null;
  bills: BillDue[];
  unreadNotifications: number;
}

export interface Notification {
  id: string;
  providerId: string;
  eventTypeId: string;
  title: string;
  body: string | null;
  url: string | null;
  readAt: string | null;
  dismissedAt: string | null;
  createdAt: string;
}

export interface NotificationsResponse {
  notifications: Notification[];
  total: number;
  limit: number;
  offset: number;
}

export interface GitHubIntegrationStatus {
  connected: boolean;
  providerUsername?: string;
  connectedAt?: string;
}

export interface GitHubRepo {
  fullName: string;  // "owner/repo" — matches backend model.GitHubRepo.FullName json:"fullName"
  private: boolean;
  watched: boolean;
}

export interface UserSettings {
  latitude: number | null;
  longitude: number | null;
  calendar_ics_url: string | null;
  timezone: string | null;
}

export interface NewsCategoryType {
  id: string;
  label: string;
  sort_order: number;
}

export interface NewsCategoriesResponse {
  available: NewsCategoryType[];
  selected: NewsCategoryType[];
}

export interface TaskLabel {
  id: string;
  name: string;
  color: string;
  created_at: string;
}
