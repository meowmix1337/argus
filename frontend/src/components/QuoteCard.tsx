import React from 'react';
import type { Quote } from '../types/dashboard';
import { Card } from './ui/Card';

interface QuoteCardProps {
  data: Quote;
  delay?: number;
  noGridSpan?: boolean;
}

export function QuoteCard({ data, delay = 0, noGridSpan = false }: QuoteCardProps): React.ReactElement {
  return (
    <Card delay={delay} noGridSpan={noGridSpan}>
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'center',
        padding: '8px 0',
      }}>
        <div style={{
          fontFamily: "'Playfair Display', Georgia, serif",
          fontSize: 20,
          fontStyle: 'italic',
          lineHeight: 1.6,
          color: 'var(--text-primary)',
          marginBottom: 16,
        }}>
          "{data.text}"
        </div>
        <div style={{
          fontSize: 13,
          color: 'var(--text-secondary)',
          fontWeight: 500,
        }}>
          — {data.author}
        </div>
      </div>
    </Card>
  );
}
