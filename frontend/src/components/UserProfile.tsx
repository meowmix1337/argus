import React, { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import type { User } from '../types/auth';

interface Props {
  user: User;
  onOpenSettings?: () => void;
  onOpenIntegrations?: () => void;
}

function getInitials(name: string): string {
  if (!name.trim()) return '?';
  return name.trim().split(' ').filter(Boolean).map((n) => n[0]).join('').toUpperCase().slice(0, 2);
}

async function signOut(
  navigate: ReturnType<typeof useNavigate>,
  clearAuth: () => void,
): Promise<void> {
  try {
    await fetch('/api/auth/logout', { method: 'POST', credentials: 'include' });
  } finally {
    clearAuth();
    navigate('/login', { replace: true });
  }
}

export function UserProfile({ user, onOpenSettings, onOpenIntegrations }: Props): React.ReactElement {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [imgError, setImgError] = useState(false);
  const [settingsHovered, setSettingsHovered] = useState(false);
  const [integrationsHovered, setIntegrationsHovered] = useState(false);
  const [signOutHovered, setSignOutHovered] = useState(false);
  const [isSigningOut, setIsSigningOut] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click or Escape
  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  const showImg = !!user.avatar_url && !imgError;

  const avatarStyle = {
    width: 36,
    height: 36,
    borderRadius: '50%',
    border: '1px solid var(--border-medium)',
    cursor: 'pointer',
    background: 'rgba(99,102,241,0.25)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 13,
    fontWeight: 600,
    color: '#c7d2fe',
    overflow: 'hidden',
    flexShrink: 0,
    outline: 'none',
  } satisfies React.CSSProperties;

  const dropdownStyle = {
    position: 'absolute',
    top: 'calc(100% + 10px)',
    right: 0,
    width: 220,
    background: 'var(--bg-primary)',
    border: '1px solid var(--border-subtle)',
    borderRadius: 16,
    backdropFilter: 'blur(20px)',
    boxShadow: '0 8px 32px rgba(0,0,0,0.4)',
    zIndex: 100,
    overflow: 'hidden',
  } satisfies React.CSSProperties;

  return (
    <div ref={containerRef} style={{ position: 'relative', display: 'inline-block', zIndex: 200 }}>
      <button
        type="button"
        aria-label="User profile"
        aria-expanded={open}
        aria-haspopup="true"
        onClick={() => setOpen((v) => !v)}
        style={avatarStyle}
      >
        {showImg ? (
          <img
            src={user.avatar_url}
            alt={user.name}
            width={36}
            height={36}
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
            onError={() => setImgError(true)}
          />
        ) : (
          getInitials(user.name)
        )}
      </button>

      {open && (
        <div role="menu" style={dropdownStyle}>
          {/* User info */}
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            padding: '14px 16px',
            borderBottom: '1px solid var(--border-subtle)',
          }}>
            <div style={{ ...avatarStyle, cursor: 'default', flexShrink: 0 }}>
              {showImg ? (
                <img
                  src={user.avatar_url}
                  alt={user.name}
                  width={36}
                  height={36}
                  style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                />
              ) : (
                getInitials(user.name)
              )}
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{
                fontSize: 13,
                fontWeight: 600,
                color: 'var(--text-primary)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}>
                {user.name}
              </div>
              <div style={{
                fontSize: 11,
                color: 'var(--text-secondary)',
                marginTop: 2,
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}>
                {user.email}
              </div>
            </div>
          </div>

          {/* Settings */}
          {onOpenSettings && (
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                onOpenSettings();
              }}
              onMouseEnter={() => setSettingsHovered(true)}
              onMouseLeave={() => setSettingsHovered(false)}
              style={{
                width: '100%',
                textAlign: 'left',
                padding: '10px 16px',
                fontSize: 13,
                color: settingsHovered ? 'var(--text-primary)' : 'var(--text-secondary)',
                background: settingsHovered ? 'var(--bg-elevated)' : 'none',
                border: 'none',
                borderBottom: '1px solid var(--border-subtle)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                transition: 'color 0.15s, background 0.15s',
              }}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="3" />
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
              </svg>
              Settings
            </button>
          )}

          {/* Integrations */}
          {onOpenIntegrations && (
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                onOpenIntegrations();
              }}
              onMouseEnter={() => setIntegrationsHovered(true)}
              onMouseLeave={() => setIntegrationsHovered(false)}
              style={{
                width: '100%',
                textAlign: 'left',
                padding: '10px 16px',
                fontSize: 13,
                color: integrationsHovered ? 'var(--text-primary)' : 'var(--text-secondary)',
                background: integrationsHovered ? 'var(--bg-elevated)' : 'none',
                border: 'none',
                borderBottom: '1px solid var(--border-subtle)',
                cursor: 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                transition: 'color 0.15s, background 0.15s',
              }}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path d="M9.5 6.5L6.5 9.5M7 4l.707-.707a3 3 0 0 1 4.243 4.243L11.243 8M9 12l-.707.707a3 3 0 0 1-4.243-4.243L4.757 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
              </svg>
              Integrations
            </button>
          )}

          {/* Sign out */}
          <button
            type="button"
            role="menuitem"
            disabled={isSigningOut}
            onClick={() => {
              setIsSigningOut(true);
              void signOut(navigate, () => queryClient.removeQueries({ queryKey: ['auth', 'me'] }));
            }}
            onMouseEnter={() => setSignOutHovered(true)}
            onMouseLeave={() => setSignOutHovered(false)}
            style={{
              width: '100%',
              textAlign: 'left',
              padding: '10px 16px',
              fontSize: 13,
              color: isSigningOut ? 'var(--text-muted)' : signOutHovered ? 'var(--text-primary)' : 'var(--text-secondary)',
              background: (!isSigningOut && signOutHovered) ? 'var(--bg-elevated)' : 'none',
              border: 'none',
              cursor: isSigningOut ? 'not-allowed' : 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              transition: 'color 0.15s, background 0.15s',
              opacity: isSigningOut ? 0.6 : 1,
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
              <polyline points="16 17 21 12 16 7" />
              <line x1="21" y1="12" x2="9" y2="12" />
            </svg>
            {isSigningOut ? 'Signing out…' : 'Sign out'}
          </button>
        </div>
      )}
    </div>
  );
}
