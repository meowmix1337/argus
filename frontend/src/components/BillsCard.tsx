import React from 'react';
import type { BillDue } from '../types/dashboard';
import { Card } from './ui/Card';

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

function formatDueDate(dateStr: string): string {
  const parts = dateStr.split('-');
  const d = new Date(parseInt(parts[0]), parseInt(parts[1]) - 1, parseInt(parts[2]));
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function formatAmount(amount: number): string {
  return amount.toLocaleString('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 0, maximumFractionDigits: 2 });
}

function currentMonthName(): string {
  return new Date().toLocaleDateString('en-US', { month: 'long' });
}

interface BillsCardProps {
  bills: BillDue[];
  delay?: number;
  noGridSpan?: boolean;
  onManage?: () => void;
}

export function BillsCard({ bills, delay = 0, noGridSpan = false, onManage }: BillsCardProps): React.ReactElement {
  const month = currentMonthName();
  const billsWithAmount = bills.filter((b) => b.amount != null);
  const total = billsWithAmount.reduce((sum, b) => sum + (b.amount ?? 0), 0);
  const hasPartialAmounts = billsWithAmount.length > 0 && billsWithAmount.length < bills.length;

  return (
    <Card delay={delay} noGridSpan={noGridSpan}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 16 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: 14, color: 'var(--text-accent)', opacity: 0.8 }}>◈</span>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-secondary)', letterSpacing: '0.04em', textTransform: 'uppercase' }}>
            {`Bills · ${month}`}
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

      {bills.length === 0 ? (
        <div style={{ fontSize: 13, color: 'var(--text-muted)', paddingTop: 4 }}>
          No bills due this month
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
                {/* Category dot */}
                <div style={{
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: color,
                  flexShrink: 0,
                }} />

                {/* Name + category */}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{
                    fontSize: 13,
                    fontWeight: 500,
                    color: 'var(--text-primary)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}>
                    {bill.name}
                  </div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 1 }}>
                    {CATEGORY_LABELS[bill.categoryId] ?? bill.categoryId}
                  </div>
                </div>

                {/* Amount (if set) */}
                {bill.amount != null && (
                  <div style={{
                    fontFamily: "'JetBrains Mono', monospace",
                    fontSize: 12,
                    color: 'var(--text-secondary)',
                    flexShrink: 0,
                  }}>
                    {formatAmount(bill.amount)}
                  </div>
                )}

                {/* Due date */}
                <div style={{
                  fontFamily: "'JetBrains Mono', monospace",
                  fontSize: 12,
                  fontWeight: 600,
                  color: 'var(--text-accent)',
                  flexShrink: 0,
                  minWidth: 44,
                  textAlign: 'right',
                }}>
                  {formatDueDate(bill.computedDueDate)}
                </div>
              </div>
            );
          })}
          {billsWithAmount.length > 0 && (
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '8px 12px 2px',
              borderTop: '1px solid rgba(255,255,255,0.06)',
              marginTop: 2,
            }}>
              <span style={{ fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                Total
              </span>
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
