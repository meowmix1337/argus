import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNotifications, useNotificationMutations } from '../hooks/useNotifications';
import { GitHubIcon } from './ui/GitHubIcon';
import type { Notification } from '../types/dashboard';

const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])';

interface NotificationPanelProps {
  onClose: () => void;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  if (isNaN(diff) || diff < 0) return 'just now';
  const m = Math.floor(diff / 60_000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 7) return `${d}d ago`;
  return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function ProviderIcon({ providerId }: { providerId: string }): React.ReactElement {
  switch (providerId) {
    case 'github':
      return <GitHubIcon size={14} style={{ color: 'var(--text-secondary)' }} />;
    default:
      return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" style={{ flexShrink: 0, color: 'var(--text-secondary)' }}>
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="8" x2="12" y2="12" />
          <line x1="12" y1="16" x2="12.01" y2="16" />
        </svg>
      );
  }
}

export function NotificationPanel({ onClose }: NotificationPanelProps): React.ReactElement {
  const [isOpen, setIsOpen] = useState(false);
  const [tab, setTab] = useState<'all' | 'unread' | 'read'>('all');
  const [page, setPage] = useState(0);
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const [allNotifications, setAllNotifications] = useState<Notification[]>([]);
  const [searchInput, setSearchInput] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');

  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const prevFocusRef = useRef<HTMLElement | null>(null);
  const closingRef = useRef(false);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const { data, isLoading } = useNotifications(tab, page, 10, debouncedQuery);
  const { markRead, markAllRead } = useNotificationMutations();

  const [prevDebouncedQuery, setPrevDebouncedQuery] = useState(debouncedQuery);
  const [prevQueryData, setPrevQueryData] = useState(data);

  // Reset pagination when search query changes (render-time setState avoids setState-in-effect lint error).
  if (debouncedQuery !== prevDebouncedQuery) {
    setPrevDebouncedQuery(debouncedQuery);
    setPage(0);
    setAllNotifications([]);
  }

  // Accumulate pages during render (avoids setState-in-effect lint error).
  // React allows calling setState during render to derive state from props/query data.
  if (data !== prevQueryData) {
    setPrevQueryData(data);
    if (data) {
      if (page === 0) {
        setAllNotifications(data.notifications);
      } else {
        setAllNotifications((prev) => {
          const existingIds = new Set(prev.map((n) => n.id));
          const newItems = data.notifications.filter((n) => !existingIds.has(n.id));
          return [...prev, ...newItems];
        });
      }
    }
  }

  const total = data?.total ?? 0;


  const handleClose = useCallback((): void => {
    if (closingRef.current) return;
    closingRef.current = true;
    setIsOpen(false);
    closeTimerRef.current = setTimeout(() => {
      prevFocusRef.current?.focus();
      onClose();
    }, 260);
  }, [onClose]);

  useEffect(() => {
    return () => clearTimeout(closeTimerRef.current);
  }, []);

  useEffect(() => {
    prevFocusRef.current = document.activeElement as HTMLElement;
    requestAnimationFrame(() => requestAnimationFrame(() => setIsOpen(true)));
  }, []);

  useEffect(() => {
    if (isOpen) closeBtnRef.current?.focus();
  }, [isOpen]);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(searchInput), 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') { handleClose(); return; }
      if (e.key === 'Tab' && panelRef.current) {
        const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE));
        if (!focusable.length) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleClose]);

  function handleBackdropClick(e: React.MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) handleClose();
  }

  function handleTabChange(newTab: 'all' | 'unread' | 'read'): void {
    setTab(newTab);
    setPage(0);
    setAllNotifications([]);
    setSearchInput('');
    setDebouncedQuery('');
  }

  function handleNotificationClick(n: Notification): void {
    if (!n.readAt) markRead.mutate(n.id);
    if (n.url) window.open(n.url, '_blank', 'noopener,noreferrer');
  }

  const tabStyle = (active: boolean): React.CSSProperties => ({
    padding: '5px 12px',
    fontSize: 12,
    fontWeight: active ? 600 : 400,
    color: active ? 'var(--text-accent)' : 'var(--text-secondary)',
    background: active ? 'rgba(99,102,241,0.15)' : 'none',
    border: active ? '1px solid rgba(99,102,241,0.3)' : '1px solid transparent',
    borderRadius: 6,
    cursor: 'pointer',
    transition: 'all 0.15s ease',
  });

  return (
    <div
      onClick={handleBackdropClick}
      style={{
        position: 'fixed', inset: 0, zIndex: 1000,
        background: isOpen ? 'rgba(0,0,0,0.35)' : 'rgba(0,0,0,0)',
        transition: 'background 0.3s ease',
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-label="Notifications"
        aria-modal="true"
        style={{
          position: 'fixed', top: 0, right: 0, bottom: 0,
          width: 440, maxWidth: '100vw',
          background: 'rgba(20,20,35,0.95)',
          borderLeft: '1px solid rgba(255,255,255,0.1)',
          backdropFilter: 'blur(24px)',
          boxShadow: isOpen
            ? '-8px 0 40px rgba(0,0,0,0.5), -1px 0 0 rgba(99,102,241,0.15)'
            : 'none',
          transform: isOpen ? 'translateX(0)' : 'translateX(100%)',
          transition: isOpen
            ? 'transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.3s ease'
            : 'transform 0.24s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.24s ease',
          display: 'flex', flexDirection: 'column', overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          padding: '20px 24px', borderBottom: '1px solid rgba(255,255,255,0.08)', flexShrink: 0,
        }}>
          <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--text-primary)' }}>
            Notifications
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <button
              type="button"
              onClick={() => markAllRead.mutate()}
              disabled={markAllRead.isPending}
              style={{
                background: 'none', border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 6, padding: '5px 10px',
                fontSize: 11, color: 'var(--text-secondary)', cursor: 'pointer',
                opacity: markAllRead.isPending ? 0.5 : 1, transition: 'opacity 0.15s',
              }}
            >
              Mark all read
            </button>
            <button
              type="button"
              ref={closeBtnRef}
              onClick={handleClose}
              style={{
                width: 32, height: 32, display: 'flex', alignItems: 'center', justifyContent: 'center',
                background: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 8, cursor: 'pointer', color: 'var(--text-secondary)', fontSize: 14,
              }}
              aria-label="Close notifications"
            >✕</button>
          </div>
        </div>

        {/* Filter tabs */}
        <div style={{
          display: 'flex', gap: 6, padding: '12px 24px',
          borderBottom: '1px solid rgba(255,255,255,0.08)', flexShrink: 0,
        }}>
          {(['all', 'unread', 'read'] as const).map((t) => (
            <button type="button" key={t} onClick={() => handleTabChange(t)} style={tabStyle(tab === t)}>
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>

        {/* Search */}
        <div style={{ padding: '8px 24px 10px', flexShrink: 0 }}>
          <div style={{ position: 'relative' }}>
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder="Search notifications..."
              style={{
                width: '100%',
                padding: '7px 12px',
                paddingRight: searchInput ? 32 : 12,
                background: 'rgba(255,255,255,0.06)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 8,
                color: 'var(--text-primary)',
                fontSize: 13,
                outline: 'none',
                boxSizing: 'border-box',
              }}
            />
            {searchInput && (
              <button
                type="button"
                onClick={() => setSearchInput('')}
                aria-label="Clear search"
                style={{
                  position: 'absolute', right: 8, top: '50%', transform: 'translateY(-50%)',
                  background: 'none', border: 'none', cursor: 'pointer',
                  color: 'var(--text-muted)', fontSize: 14, padding: 0,
                  display: 'flex', alignItems: 'center',
                }}
              >✕</button>
            )}
          </div>
        </div>

        {/* Notification list */}
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px 0' }}>
          {isLoading && allNotifications.length === 0 ? (
            <div style={{ padding: 24, fontSize: 13, color: 'var(--text-muted)' }}>Loading...</div>
          ) : allNotifications.length === 0 ? (
            <div style={{ padding: 32, textAlign: 'center', fontSize: 13, color: 'var(--text-muted)' }}>
              {debouncedQuery ? `No results for "${debouncedQuery}"` : 'No notifications'}
            </div>
          ) : (
            <>
              {allNotifications.map((n) => (
                <div
                  key={n.id}
                  onMouseEnter={() => setHoveredId(n.id)}
                  onMouseLeave={() => setHoveredId(null)}
                  style={{
                    display: 'flex', alignItems: 'flex-start', gap: 10,
                    padding: '10px 24px',
                    background: !n.readAt
                      ? hoveredId === n.id ? 'rgba(99,102,241,0.10)' : 'rgba(99,102,241,0.06)'
                      : hoveredId === n.id ? 'rgba(255,255,255,0.04)' : 'transparent',
                    transition: 'background 0.15s ease',
                    borderLeft: !n.readAt ? '2px solid rgba(99,102,241,0.7)' : '2px solid transparent',
                    position: 'relative',
                  }}
                >
                  {/* Provider icon */}
                  <div style={{ paddingTop: 2 }}>
                    <ProviderIcon providerId={n.providerId} />
                  </div>

                  {/* Content */}
                  <div style={{ flex: 1, minWidth: 0 }}>
                    {n.url ? (
                      <button
                        type="button"
                        onClick={() => handleNotificationClick(n)}
                        style={{
                          background: 'none', border: 'none', padding: 0,
                          fontSize: 13, fontWeight: n.readAt ? 400 : 600,
                          color: n.readAt ? 'var(--text-secondary)' : 'var(--text-primary)',
                          cursor: 'pointer', textAlign: 'left',
                          display: 'block', width: '100%',
                          overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                        }}
                      >
                        {n.title}
                      </button>
                    ) : (
                      <div style={{
                        fontSize: 13, fontWeight: n.readAt ? 400 : 600,
                        color: n.readAt ? 'var(--text-secondary)' : 'var(--text-primary)',
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}>
                        {n.title}
                      </div>
                    )}
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, marginTop: 3 }}>
                      <span style={{
                        fontSize: 10, color: 'var(--text-muted)',
                        background: 'rgba(255,255,255,0.06)',
                        borderRadius: 4, padding: '1px 5px',
                      }}>
                        {n.eventTypeId.replace(/_/g, ' ')}
                      </span>
                      <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>
                        {relativeTime(n.createdAt)}
                      </span>
                    </div>
                  </div>

                </div>
              ))}

              {/* Load more */}
              {allNotifications.length < total && (
                <div style={{ padding: '12px 24px' }}>
                  <button
                    type="button"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={isLoading}
                    style={{
                      width: '100%', padding: '8px',
                      background: 'rgba(255,255,255,0.05)',
                      border: '1px solid rgba(255,255,255,0.1)',
                      borderRadius: 8, fontSize: 12,
                      color: 'var(--text-secondary)', cursor: isLoading ? 'not-allowed' : 'pointer',
                      opacity: isLoading ? 0.5 : 1,
                    }}
                  >
                    {isLoading ? 'Loading...' : 'Load more'}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}
