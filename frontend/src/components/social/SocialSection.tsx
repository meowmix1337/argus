import React, { useState } from 'react';
import { PostComposer } from './PostComposer';
import { FeedList } from './FeedList';
import { SearchBar } from './SearchBar';
import { DiscoverPeople } from './DiscoverPeople';

type Tab = 'feed' | 'search';

interface SocialSectionProps {
  isMobile?: boolean;
}

export function SocialSection({ isMobile = false }: SocialSectionProps): React.ReactElement {
  const [tab, setTab] = useState<Tab>('feed');

  return (
    <div style={{ marginTop: isMobile ? 24 : 40 }}>
      {/* Section header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        marginBottom: 20,
        paddingBottom: 16,
        borderBottom: '1px solid var(--header-border)',
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 14, color: 'var(--text-accent)', opacity: 0.8 }}>◈</span>
          <span style={{
            fontSize: 13,
            fontWeight: 600,
            color: 'var(--text-secondary)',
            letterSpacing: '0.04em',
            textTransform: 'uppercase',
          }}>
            Social
          </span>
        </div>

        {/* Tab switcher */}
        <div style={{ display: 'flex', gap: 4 }}>
          {(['feed', 'search'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              style={{
                background: tab === t ? 'rgba(99,102,241,0.18)' : 'transparent',
                border: `1px solid ${tab === t ? 'rgba(99,102,241,0.4)' : 'transparent'}`,
                borderRadius: 7,
                padding: '4px 12px',
                fontSize: 12,
                fontWeight: 600,
                color: tab === t ? 'var(--text-accent)' : 'var(--text-muted)',
                cursor: 'pointer',
                transition: 'all 0.15s',
                textTransform: 'capitalize',
              }}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* Two-column layout on desktop, single column on mobile */}
      <div style={{
        display: 'grid',
        gridTemplateColumns: isMobile ? '1fr' : '1fr 280px',
        gap: isMobile ? 16 : 24,
        alignItems: 'start',
      }}>
        {/* Main column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {tab === 'feed' && (
            <>
              <PostComposer />
              <div style={{
                background: 'var(--bg-card)',
                border: '1px solid var(--bg-card-border)',
                borderRadius: 12,
                padding: '0 16px',
                backdropFilter: 'blur(20px)',
              }}>
                <FeedList />
              </div>
            </>
          )}
          {tab === 'search' && <SearchBar />}
        </div>

        {/* Sidebar — hidden on mobile */}
        {!isMobile && (
          <div>
            <DiscoverPeople />
          </div>
        )}
      </div>
    </div>
  );
}
