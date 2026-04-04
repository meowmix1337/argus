import React, { useState } from 'react';
import { useNews } from '../hooks/useNews';
import { Card } from './ui/Card';
import { CardHeader } from './ui/CardHeader';

interface NewsCardProps {
  delay?: number;
  isMobile?: boolean;
  isTablet?: boolean;
  noGridSpan?: boolean;
}

function titleCase(s: string): string {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function Skeleton({ width = '100%', height = 16 }: { width?: string | number; height?: number }): React.ReactElement {
  return (
    <div style={{
      width,
      height,
      borderRadius: 4,
      background: 'var(--bg-skeleton)',
      animation: 'pulse 1.5s ease-in-out infinite',
    }} />
  );
}

export function NewsCard({ delay = 0, isMobile = false, isTablet = false, noGridSpan = false }: NewsCardProps): React.ReactElement {
  const { data: categories, isLoading, isError, refetch, isFetching } = useNews();
  const [activeIndex, setActiveIndex] = useState(0);

  const active = categories?.[activeIndex] ?? { name: '', items: [] };

  return (
    <Card delay={delay} span={isMobile || isTablet ? 1 : 2} noGridSpan={noGridSpan}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <CardHeader icon="⊞" title="Headlines" />
        <button
          onClick={() => refetch()}
          disabled={isFetching}
          style={{
            padding: '4px 10px',
            borderRadius: 6,
            border: '1px solid var(--border-subtle)',
            background: 'var(--bg-card)',
            color: isFetching ? 'var(--text-muted)' : 'var(--text-secondary)',
            fontSize: 12,
            cursor: isFetching ? 'not-allowed' : 'pointer',
            transition: 'all 0.15s',
          }}
        >
          {isFetching ? 'Refreshing…' : '↻ Refresh'}
        </button>
      </div>

      {isLoading ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} width={60} height={26} />
            ))}
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr', gap: 12 }}>
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '14px 16px', borderRadius: 10, background: 'var(--bg-card)', border: '1px solid var(--border-subtle)' }}>
                <Skeleton width="30%" height={11} />
                <Skeleton width="100%" height={14} />
                <Skeleton width="80%" height={14} />
                <Skeleton width="40%" height={12} />
              </div>
            ))}
          </div>
        </div>
      ) : isError || !categories ? (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 160, color: 'var(--text-muted)', fontSize: 13 }}>
          News unavailable
        </div>
      ) : (
        <>
          <div style={{
            display: 'flex',
            gap: 6,
            marginBottom: 16,
            overflowX: 'auto',
            scrollbarWidth: 'none',
            flexWrap: 'nowrap',
            paddingBottom: 2,
          }}>
            {categories.map((cat, i) => {
              const isActive = i === activeIndex;
              return (
                <button
                  key={cat.name}
                  onClick={() => setActiveIndex(i)}
                  style={{
                    flexShrink: 0,
                    padding: '5px 13px',
                    borderRadius: 20,
                    border: isActive ? '1px solid rgba(99,102,241,0.4)' : '1px solid var(--border-subtle)',
                    background: isActive ? 'rgba(99,102,241,0.2)' : 'var(--bg-card)',
                    color: isActive ? '#818cf8' : 'var(--text-secondary)',
                    fontSize: 12,
                    fontWeight: isActive ? 600 : 400,
                    cursor: 'pointer',
                    transition: 'all 0.15s',
                    letterSpacing: '0.02em',
                    lineHeight: 1,
                  }}
                >
                  {titleCase(cat.name)}
                </button>
              );
            })}
          </div>

          {active.items.length === 0 ? (
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 120, color: 'var(--text-muted)', fontSize: 13 }}>
              No articles available
            </div>
          ) : (
            <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', scrollbarWidth: 'thin', scrollbarColor: 'rgba(99,102,241,0.25) transparent' }}>
              <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : '1fr 1fr', gap: 12 }}>
                {active.items.map((item, i) => (
                  <div
                    key={i}
                    onClick={() => item.url && window.open(item.url, '_blank', 'noopener,noreferrer')}
                    style={{
                      padding: '14px 16px',
                      borderRadius: 10,
                      background: 'var(--bg-card)',
                      border: '1px solid var(--border-subtle)',
                      cursor: item.url ? 'pointer' : 'default',
                      transition: 'all 0.2s',
                    }}
                  >
                    <div style={{ marginBottom: 8 }}>
                      <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{item.time}</span>
                    </div>
                    <div style={{ fontSize: 14, fontWeight: 500, color: 'var(--text-primary)', lineHeight: 1.45 }}>
                      {item.title}
                    </div>
                    <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 6 }}>
                      {item.source}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </Card>
  );
}
