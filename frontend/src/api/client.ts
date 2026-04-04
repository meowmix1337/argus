import type { Bill, BillDue, DashboardResponse, NewsCategory, Task, StockQuote, SymbolSearchResult, TaskLabel, UserSettings, NewsCategoriesResponse, NotificationsResponse, GitHubIntegrationStatus, GitHubRepo } from '../types/dashboard';

export interface BillsListResponse {
  bills: Bill[];
  total: number;
  limit: number;
  offset: number;
}

export interface BillPayload {
  name: string;
  amount?: number | null;
  categoryId: string;
  recurrenceType: string;
  dueDate?: string | null;
  dueDay?: number | null;
  dueMonth?: number | null;
  anchorDate?: string | null;
  notes?: string | null;
}

const BASE = '/api';

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const { headers: extraHeaders, ...rest } = options ?? {};
  const res = await fetch(`${BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(extraHeaders as Record<string, string>) },
    ...rest,
  });
  if (!res.ok) {
    throw new Error(`API error ${res.status}: ${res.statusText}`);
  }
  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export function fetchDashboard(): Promise<DashboardResponse> {
  return apiFetch<DashboardResponse>('/dashboard');
}

export function fetchNews(): Promise<NewsCategory[]> {
  return apiFetch<NewsCategory[]>('/news');
}

export function toggleTask(id: string, done: boolean): Promise<Task> {
  return apiFetch<Task>(`/tasks/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ done }),
  });
}

export function createTask(text: string, priority: string): Promise<Task> {
  return apiFetch<Task>('/tasks', {
    method: 'POST',
    body: JSON.stringify({ text, priority }),
  });
}

export function fetchStocks(): Promise<StockQuote[]> {
  return apiFetch<StockQuote[]>('/stocks');
}

export function addStockSymbol(symbol: string): Promise<{ symbols: string[] }> {
  return apiFetch<{ symbols: string[] }>('/stocks/watchlist', {
    method: 'POST',
    body: JSON.stringify({ symbol }),
  });
}

export function removeStockSymbol(symbol: string): Promise<{ symbols: string[] }> {
  return apiFetch<{ symbols: string[] }>(`/stocks/watchlist/${encodeURIComponent(symbol)}`, {
    method: 'DELETE',
  });
}

export function searchSymbols(query: string): Promise<{ results: SymbolSearchResult[] }> {
  return apiFetch<{ results: SymbolSearchResult[] }>(`/stocks/search?q=${encodeURIComponent(query)}`);
}

export interface TasksPageResponse {
  tasks: Task[];
  total: number;
  limit: number;
  offset: number;
}

export function fetchTasksPage(limit: number, offset: number): Promise<TasksPageResponse> {
  return apiFetch<TasksPageResponse>(`/tasks?limit=${limit}&offset=${offset}`);
}

export function deleteTask(id: string): Promise<void> {
  return apiFetch(`/tasks/${id}`, { method: 'DELETE' }).then(() => undefined);
}

export function fetchUserSettings(): Promise<UserSettings> {
  return apiFetch<UserSettings>('/settings');
}

export function upsertUserSettings(settings: Partial<UserSettings>): Promise<UserSettings> {
  return apiFetch<UserSettings>('/settings', { method: 'PUT', body: JSON.stringify(settings) });
}

export function fetchNewsCategories(): Promise<NewsCategoriesResponse> {
  return apiFetch<NewsCategoriesResponse>('/settings/news-categories');
}

export function setNewsCategories(categoryIds: string[]): Promise<void> {
  return apiFetch('/settings/news-categories', { method: 'PUT', body: JSON.stringify({ category_ids: categoryIds }) }).then(() => undefined);
}
export function fetchBills(limit = 50, offset = 0): Promise<BillsListResponse> {
  return apiFetch<BillsListResponse>(`/bills?limit=${limit}&offset=${offset}`);
}
export function createBill(payload: BillPayload): Promise<Bill> {
  return apiFetch<Bill>('/bills', { method: 'POST', body: JSON.stringify(payload) });
}
export function updateBill(id: string, payload: BillPayload): Promise<Bill> {
  return apiFetch<Bill>(`/bills/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify(payload) });
}
export function deleteBill(id: string): Promise<void> {
  return apiFetch(`/bills/${encodeURIComponent(id)}`, { method: 'DELETE' }).then(() => undefined);
}
export interface BillsDueResponse {
  bills: BillDue[];
  year: number;
  month: number;
}
export function fetchBillsDue(year: number, month: number): Promise<BillsDueResponse> {
  return apiFetch<BillsDueResponse>(`/bills/due?year=${year}&month=${month}`);
}
export interface BillsMonthEntry {
  month: number;
  bills: BillDue[];
}
export interface BillsYearResponse {
  year: number;
  months: BillsMonthEntry[];
}
export function fetchBillsDueYear(year: number): Promise<BillsYearResponse> {
  return apiFetch<BillsYearResponse>(`/bills/due/year?year=${year}`);
}
export interface BillPaymentResponse {
  id: string;
  billId: string;
  computedDueDate: string;
  paidDate: string;
  note: string | null;
  createdAt: string;
}
export interface MarkPaidPayload {
  computedDueDate: string;
  paidDate: string;
  note?: string | null;
}
export function markBillPaid(billId: string, payload: MarkPaidPayload): Promise<BillPaymentResponse> {
  return apiFetch<BillPaymentResponse>(`/bills/${encodeURIComponent(billId)}/pay`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}
export function unmarkBillPaid(paymentId: string): Promise<void> {
  return apiFetch(`/bills/payments/${encodeURIComponent(paymentId)}`, { method: 'DELETE' }).then(() => undefined);
}
export function fetchLabels(): Promise<TaskLabel[]> {
  return apiFetch<TaskLabel[]>('/labels');
}
export function createLabel(name: string, color: string): Promise<TaskLabel> {
  return apiFetch<TaskLabel>('/labels', { method: 'POST', body: JSON.stringify({ name, color }) });
}
export function updateLabel(id: string, name?: string, color?: string): Promise<TaskLabel> {
  return apiFetch<TaskLabel>(`/labels/${id}`, { method: 'PATCH', body: JSON.stringify({ name, color }) });
}
export function deleteLabel(id: string): Promise<void> {
  return apiFetch(`/labels/${id}`, { method: 'DELETE' }).then(() => undefined);
}
export function fetchTaskLabels(taskId: string): Promise<TaskLabel[]> {
  return apiFetch<TaskLabel[]>(`/tasks/${encodeURIComponent(taskId)}/labels`);
}
export function assignLabelToTask(taskId: string, labelId: string): Promise<void> {
  return apiFetch(`/tasks/${encodeURIComponent(taskId)}/labels`, { method: 'POST', body: JSON.stringify({ label_id: labelId }) }).then(() => undefined);
}
export function removeLabelFromTask(taskId: string, labelId: string): Promise<void> {
  return apiFetch(`/tasks/${encodeURIComponent(taskId)}/labels/${encodeURIComponent(labelId)}`, { method: 'DELETE' }).then(() => undefined);
}

// Notifications
export function fetchNotifications(state = 'all', limit = 20, offset = 0, query = '', providerID = '', eventTypeID = ''): Promise<NotificationsResponse> {
  let url = `/notifications?state=${encodeURIComponent(state)}&limit=${limit}&offset=${offset}`;
  if (query) url += `&q=${encodeURIComponent(query)}`;
  if (providerID) url += `&provider=${encodeURIComponent(providerID)}`;
  if (eventTypeID) url += `&event_type=${encodeURIComponent(eventTypeID)}`;
  return apiFetch<NotificationsResponse>(url);
}
export function markNotificationRead(id: string): Promise<void> {
  return apiFetch(`/notifications/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ action: 'read' }) }).then(() => undefined);
}
export function markAllNotificationsRead(): Promise<void> {
  return apiFetch('/notifications/mark-all-read', { method: 'POST' }).then(() => undefined);
}
export function fetchUnreadCount(): Promise<{ count: number }> {
  return apiFetch<NotificationsResponse>('/notifications?state=unread&limit=1&offset=0').then((r) => ({ count: r.total }));
}

// GitHub Integration
export function getGitHubIntegrationStatus(): Promise<GitHubIntegrationStatus> {
  return apiFetch<GitHubIntegrationStatus>('/integrations/github');
}
export function disconnectGitHub(): Promise<void> {
  return apiFetch('/integrations/github', { method: 'DELETE' }).then(() => undefined);
}
export function fetchGitHubRepos(): Promise<GitHubRepo[]> {
  return apiFetch<GitHubRepo[]>('/integrations/github/repos');
}
export function updateWatchedRepos(repos: string[]): Promise<void> {
  return apiFetch('/integrations/github/repos', { method: 'PUT', body: JSON.stringify({ repos }) }).then(() => undefined);
}
