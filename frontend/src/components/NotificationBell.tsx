import React, { useState } from 'react';

interface NotificationBellProps {
  unreadCount: number;
  onClick: () => void;
  isOpen: boolean;
}

export function NotificationBell({ unreadCount, onClick, isOpen }: NotificationBellProps): React.ReactElement {
  const [hovered, setHovered] = useState(false);
  const badge = unreadCount > 9 ? '9+' : String(unreadCount);

  return (
    <button
      type="button"
      aria-label={`Notifications${unreadCount > 0 ? `, ${unreadCount} unread` : ''}`}
      aria-pressed={isOpen}
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        position: 'relative',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 36,
        height: 36,
        borderRadius: 8,
        background: isOpen
          ? 'rgba(99,102,241,0.2)'
          : hovered
          ? 'var(--toggle-hover-bg)'
          : 'var(--toggle-bg)',
        border: isOpen
          ? '1px solid rgba(99,102,241,0.4)'
          : '1px solid var(--toggle-border)',
        color: isOpen ? 'rgba(165,180,252,1)' : 'var(--toggle-text)',
        cursor: 'pointer',
        flexShrink: 0,
        transition: 'background 0.2s ease, border-color 0.2s ease, color 0.2s ease',
      }}
    >
      <svg
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
        <path d="M13.73 21a2 2 0 0 1-3.46 0" />
      </svg>

      {unreadCount > 0 && (
        <span
          aria-hidden="true"
          style={{
            position: 'absolute',
            top: -4,
            right: -4,
            minWidth: 16,
            height: 16,
            borderRadius: 8,
            background: '#ef4444',
            color: '#fff',
            fontSize: 10,
            fontWeight: 700,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            padding: '0 3px',
            lineHeight: 1,
            border: '1.5px solid var(--bg-primary)',
          }}
        >
          {badge}
        </span>
      )}
    </button>
  );
}
