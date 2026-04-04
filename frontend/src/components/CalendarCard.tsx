import React from 'react';
import type { CalendarEvent } from '../types/dashboard';
import { Card } from './ui/Card';
import { CardHeader } from './ui/CardHeader';

function formatEventTime(raw: string): string {
  if (raw === 'All Day') return raw;
  const d = new Date(raw);
  if (isNaN(d.getTime())) return raw;
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
}

interface CalendarCardProps {
  events: CalendarEvent[];
  delay?: number;
  noGridSpan?: boolean;
}

export function CalendarCard({ events, delay = 0, noGridSpan = false }: CalendarCardProps): React.ReactElement {
  const now = new Date();

  const nextIndex = events.findIndex(e => {
    if (e.time === 'All Day') return false;
    const t = new Date(e.time);
    return !isNaN(t.getTime()) && t > now;
  });

  function isPast(event: CalendarEvent): boolean {
    if (event.time === 'All Day') return false;
    const t = new Date(event.time);
    return !isNaN(t.getTime()) && t < now;
  }

  return (
    <Card delay={delay} noGridSpan={noGridSpan}>
      <CardHeader icon="▦" title="Today's Schedule" badge={`${events.length} events`} />
      <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: 8, scrollbarWidth: 'thin', scrollbarColor: 'rgba(99,102,241,0.25) transparent' }}>
        {events.map((event, i) => {
          const past = isPast(event);
          const isNext = nextIndex !== -1 && i === nextIndex;

          return (
            <div
              key={i}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                padding: '10px 12px',
                borderRadius: 10,
                background: isNext ? 'rgba(99,102,241,0.1)' : 'var(--bg-card)',
                border: isNext
                  ? '1px solid rgba(99,102,241,0.2)'
                  : '1px solid transparent',
                opacity: past ? 0.45 : 1,
                transition: 'all 0.2s',
              }}
            >
              <div style={{
                width: 3,
                height: 32,
                borderRadius: 2,
                background: event.color,
                flexShrink: 0,
              }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{
                  fontSize: 14,
                  fontWeight: 500,
                  color: 'var(--text-primary)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}>
                  {event.title}
                </div>
                <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginTop: 2 }}>
                  {formatEventTime(event.time)} · {event.duration}
                </div>
              </div>
              {isNext && (
                <div style={{
                  fontSize: 10,
                  fontWeight: 600,
                  color: 'var(--text-accent)',
                  background: 'rgba(99,102,241,0.15)',
                  padding: '3px 8px',
                  borderRadius: 20,
                  letterSpacing: '0.05em',
                  textTransform: 'uppercase',
                  flexShrink: 0,
                }}>
                  Next
                </div>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}
