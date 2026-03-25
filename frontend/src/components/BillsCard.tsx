import React, { useState } from 'react';
import { Card } from './ui/Card';
import { useBillsDueYear } from '../hooks/useBillsDueYear';

const CATEGORY_LABELS: Record<string, string> = {
  rent: 'Rent / Mortgage',
  utilities: 'Utilities',
  subscriptions: 'Subscriptions',
  insurance: 'Insurance',
  loans: 'Loans',
  medical: 'Medical',
  other: 'Other',
};

const CATEGORY_COLORS: Record<string, string> = {
  rent: '#f97316',
  utilities: '#3b82f6',
  subscriptions: '#8b5cf6',
  insurance: '#ec4899',
  loans: '#ef4444',
  medical: '#10b981',
  other: '#6b7280',
};

const MONTH_NAMES = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const MONTH_NAMES_FULL = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function formatDueDate(dateStr: string): string {
  const parts = dateStr.split('-');
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]));
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function formatAmount(amount: number): string {
  return amount.toLocaleString('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 0, maximumFractionDigits: 2 });
}

interface BillsCardProps {
  delay?: number;
  noGridSpan?: boolean;
  onManage?: () => void;
}

export function BillsCard({ delay = 0, noGridSpan = false, onManage }: BillsCardProps): React.ReactElement {
  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonthIdx = now.getMonth(); // 0-based

  const [selectedMonthIdx, setSelectedMonthIdx] = useState(currentMonthIdx);
  const { monthBills, isLoading } = useBillsDueYear(currentYear);
  const bills = monthBills[selectedMonthIdx + 1] ?? [];

  const billsWithAmount = bills.filter((b) => b.amount != null);
  const total = billsWithAmount.reduce((sum, b) => sum + (b.amount ?? 0), 0);
  const hasPartialAmounts = billsWithAmount.length > 0 && billsWithAmount.length < bills.length;

  const canGoPrev = selectedMonthIdx > 0;
  const canGoNext = selectedMonthIdx < 11;

  const arrowStyle = (enabled: boolean): React.CSSProperties => ({
    background: 'none',
    border: 'none',
    cursor: enabled ? 'pointer' : 'default',
    color: enabled ? 'var(--text-secondary)' : 'rgba(255,255,255,0.15)',
    fontSize: 14,
    padding: '0 4px',
    lineHeight: 1,
    flexShrink: 0,
  });

  return (
    <Card delay={delay} noGridSpan={noGridSpan}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 14, color: 'var(--text-accent)', opacity: 0.8 }}>◈</span>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-secondary)', letterSpacing: '0.04em', textTransform: 'uppercase' }}>
            Bills
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {bills.length > 0 && (
            <span style={{ fontSize: 11, color: 'var(--text-secondary)', fontFamily: "'JetBrains Mono', monospace", fontWeight: 400 }}>
              {bills.length} due
            </span>
          )}
          {onManage && (
            <button
              onClick={onManage}
              style={{
                background: 'rgba(255,255,255,0.05)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 6,
                padding: '3px 10px',
                fontSize: 11,
                color: 'var(--text-secondary)',
                cursor: 'pointer',
                flexShrink: 0,
              }}
            >
              Manage
            </button>
          )}
        </div>
      </div>

      {/* Month carousel */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 14 }}>
        <button
          onClick={() => canGoPrev && setSelectedMonthIdx((m) => m - 1)}
          disabled={!canGoPrev}
          aria-label="Previous month"
          style={arrowStyle(canGoPrev)}
        >
          ‹
        </button>
        <div style={{ textAlign: 'center' }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
            {MONTH_NAMES_FULL[selectedMonthIdx]}
          </span>
          {selectedMonthIdx !== currentMonthIdx && (
            <button
              onClick={() => setSelectedMonthIdx(currentMonthIdx)}
              style={{
                marginLeft: 8,
                background: 'none',
                border: 'none',
                fontSize: 10,
                color: 'var(--text-accent)',
                cursor: 'pointer',
                padding: 0,
                textDecoration: 'underline',
              }}
            >
              today
            </button>
          )}
        </div>
        <button
          onClick={() => canGoNext && setSelectedMonthIdx((m) => m + 1)}
          disabled={!canGoNext}
          aria-label="Next month"
          style={arrowStyle(canGoNext)}
        >
          ›
        </button>
      </div>

      {/* Mini month strip */}
      <div style={{ display: 'flex', gap: 2, marginBottom: 16 }}>
        {MONTH_NAMES.map((name, idx) => (
          <button
            key={name}
            onClick={() => setSelectedMonthIdx(idx)}
            aria-label={MONTH_NAMES_FULL[idx]}
            style={{
              flex: 1,
              background: idx === selectedMonthIdx ? 'rgba(99,102,241,0.25)' : 'none',
              border: 'none',
              borderRadius: 4,
              padding: '3px 0',
              fontSize: 9,
              fontWeight: idx === selectedMonthIdx ? 700 : 400,
              color: idx === selectedMonthIdx ? 'var(--text-accent)' : idx === currentMonthIdx ? 'var(--text-secondary)' : 'rgba(255,255,255,0.2)',
              cursor: 'pointer',
              letterSpacing: '0.02em',
            }}
          >
            {name}
          </button>
        ))}
      </div>

      {/* Bills list */}
      {isLoading ? (
        <div style={{ fontSize: 12, color: 'var(--text-muted)', paddingTop: 4 }}>Loading…</div>
      ) : bills.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--text-muted)', paddingTop: 4 }}>
          No bills due in {MONTH_NAMES_FULL[selectedMonthIdx]}
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {bills.map((bill) => {
            const color = CATEGORY_COLORS[bill.categoryId] ?? CATEGORY_COLORS.other;
            return (
              <div
                key={bill.id}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '9px 12px',
                  borderRadius: 10,
                  background: 'var(--bg-card)',
                  border: '1px solid transparent',
                  transition: 'background 0.15s',
                }}
              >
                <div style={{ width: 8, height: 8, borderRadius: '50%', background: color, flexShrink: 0 }} />

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {bill.name}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 1 }}>
                    {CATEGORY_LABELS[bill.categoryId] ?? bill.categoryId}
                  </div>
                </div>

                {bill.amount != null && (
                  <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, color: 'var(--text-secondary)', flexShrink: 0 }}>
                    {formatAmount(bill.amount)}
                  </div>
                )}

                <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, fontWeight: 600, color: 'var(--text-accent)', flexShrink: 0, minWidth: 44, textAlign: 'right' }}>
                  {formatDueDate(bill.computedDueDate)}
                </div>
              </div>
            );
          })}

          {billsWithAmount.length > 0 && (
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 12px 2px', borderTop: '1px solid rgba(255,255,255,0.06)', marginTop: 2 }}>
              <span style={{ fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Total</span>
              <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                {formatAmount(total)}{hasPartialAmounts ? '+' : ''}
              </span>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}
