import React, { useState, useMemo } from 'react';
import { Card } from './ui/Card';
import { useBillsDueYear } from '../hooks/useBillsDueYear';

function todayStr(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

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

function EyeIcon({ size = 13 }: { size?: number }): React.ReactElement {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"
      aria-hidden="true">
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  );
}

function EyeOffIcon({ size = 13 }: { size?: number }): React.ReactElement {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round"
      aria-hidden="true">
      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
      <line x1="1" y1="1" x2="23" y2="23" />
    </svg>
  );
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
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [formDate, setFormDate] = useState(todayStr());
  const [formNote, setFormNote] = useState('');
  const [amountsVisible, setAmountsVisible] = useState(false);

  const { monthBills, isLoading, markPaid, unmark, isPending } = useBillsDueYear(currentYear);
  const bills = monthBills[selectedMonthIdx + 1] ?? [];

  const billsWithAmount = bills.filter((b) => b.amount != null);
  const total = billsWithAmount.reduce((sum, b) => sum + (b.amount ?? 0), 0);
  const hasPartialAmounts = billsWithAmount.length > 0 && billsWithAmount.length < bills.length;

  // Annual total across all months in the year — memoized to avoid recomputing on every render
  const { annualTotal, hasPartialAnnual, annualBillsWithAmount } = useMemo(() => {
    const all = Object.values(monthBills).flat();
    const withAmount = all.filter((b) => b.amount != null);
    return {
      annualTotal: withAmount.reduce((sum, b) => sum + (b.amount ?? 0), 0),
      hasPartialAnnual: withAmount.length > 0 && withAmount.length < all.length,
      annualBillsWithAmount: withAmount,
    };
  }, [monthBills]);

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
          <button
            onClick={() => setAmountsVisible((v) => !v)}
            aria-label={amountsVisible ? 'Hide amounts' : 'Show amounts'}
            title={amountsVisible ? 'Hide amounts' : 'Show amounts'}
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: amountsVisible ? 'var(--text-accent)' : 'rgba(255,255,255,0.3)',
              padding: '2px 4px',
              display: 'flex',
              alignItems: 'center',
              transition: 'color 0.15s',
            }}
          >
            {amountsVisible ? <EyeIcon /> : <EyeOffIcon />}
          </button>
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
        <>
          {/* Scrollable list — total is NOT inside here */}
          <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', scrollbarWidth: 'thin', scrollbarColor: 'rgba(99,102,241,0.25) transparent', WebkitMaskImage: 'linear-gradient(to bottom, black calc(100% - 24px), transparent 100%)', maskImage: 'linear-gradient(to bottom, black calc(100% - 24px), transparent 100%)' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {bills.map((bill) => {
                const dotColor = bill.isPaid ? '#22c55e' : (CATEGORY_COLORS[bill.categoryId] ?? CATEGORY_COLORS.other);
                const isExpanded = expandedId === bill.id;
                return (
                  <div
                    key={bill.id}
                    style={{
                      borderRadius: 10,
                      background: bill.isPaid ? 'rgba(34,197,94,0.06)' : 'var(--bg-card)',
                      border: `1px solid ${bill.isPaid ? 'rgba(34,197,94,0.15)' : 'transparent'}`,
                      overflow: 'hidden',
                      transition: 'background 0.15s',
                    }}
                  >
                    {/* Main row */}
                    <div
                      onClick={() => {
                        if (bill.isPaid) {
                          if (bill.paymentId) unmark(bill.paymentId, bill.id, bill.computedDueDate);
                        } else {
                          if (isExpanded) {
                            setExpandedId(null);
                          } else {
                            setExpandedId(bill.id);
                            setFormDate(todayStr());
                            setFormNote('');
                          }
                        }
                      }}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 10,
                        padding: '9px 12px',
                        cursor: isPending ? 'not-allowed' : 'pointer',
                        opacity: isPending ? 0.6 : 1,
                      }}
                    >
                      <div style={{ width: 8, height: 8, borderRadius: '50%', background: dotColor, flexShrink: 0 }} />

                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {bill.name}
                        </div>
                        <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 1 }}>
                          {bill.isPaid && bill.paidNote ? bill.paidNote : (CATEGORY_LABELS[bill.categoryId] ?? bill.categoryId)}
                        </div>
                      </div>

                      {bill.amount != null && (
                        <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, color: 'var(--text-secondary)', flexShrink: 0, minWidth: 52, textAlign: 'right' }}>
                          {amountsVisible ? formatAmount(bill.amount) : '••••'}
                        </div>
                      )}

                      <div style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 12, fontWeight: 600, color: bill.isPaid ? '#22c55e' : 'var(--text-accent)', flexShrink: 0, minWidth: 44, textAlign: 'right' }}>
                        {formatDueDate(bill.computedDueDate)}
                      </div>
                    </div>

                    {/* Inline mark-paid form */}
                    {!bill.isPaid && isExpanded && (
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 8,
                          padding: '8px 12px 10px 30px',
                          borderTop: '1px solid rgba(255,255,255,0.06)',
                          flexWrap: 'wrap',
                        }}
                      >
                        <input
                          type="date"
                          value={formDate}
                          onChange={(e) => setFormDate(e.target.value)}
                          style={{
                            background: 'rgba(255,255,255,0.07)',
                            border: '1px solid rgba(255,255,255,0.12)',
                            borderRadius: 6,
                            padding: '4px 8px',
                            fontSize: 12,
                            color: 'var(--text-primary)',
                            colorScheme: 'dark',
                          }}
                        />
                        <input
                          type="text"
                          value={formNote}
                          onChange={(e) => setFormNote(e.target.value.slice(0, 32))}
                          placeholder="Note (optional)"
                          maxLength={32}
                          style={{
                            flex: 1,
                            minWidth: 80,
                            background: 'rgba(255,255,255,0.07)',
                            border: '1px solid rgba(255,255,255,0.12)',
                            borderRadius: 6,
                            padding: '4px 8px',
                            fontSize: 12,
                            color: 'var(--text-primary)',
                          }}
                        />
                        <button
                          disabled={isPending || !formDate}
                          onClick={(e) => {
                            e.stopPropagation();
                            markPaid(bill.id, {
                              computedDueDate: bill.computedDueDate,
                              paidDate: formDate,
                              note: formNote.trim() || null,
                            });
                            setExpandedId(null);
                            setFormDate(todayStr());
                            setFormNote('');
                          }}
                          style={{
                            background: 'rgba(34,197,94,0.2)',
                            border: '1px solid rgba(34,197,94,0.35)',
                            borderRadius: 6,
                            padding: '4px 12px',
                            fontSize: 12,
                            color: '#22c55e',
                            cursor: isPending || !formDate ? 'not-allowed' : 'pointer',
                            fontWeight: 500,
                            flexShrink: 0,
                          }}
                        >
                          ✓ Mark paid
                        </button>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          {/* Pinned footer — monthly + annual totals, always visible */}
          {(billsWithAmount.length > 0 || annualBillsWithAmount.length > 0) && (
            <div style={{
              display: 'flex',
              justifyContent: billsWithAmount.length > 0 && annualBillsWithAmount.length > 0 ? 'space-between' : 'flex-end',
              alignItems: 'center',
              padding: '8px 12px',
              borderTop: '1px solid rgba(255,255,255,0.10)',
              marginTop: 4,
              flexShrink: 0,
            }}>
              {billsWithAmount.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  <span style={{ fontSize: 9, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Monthly</span>
                  <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                    {amountsVisible ? `${formatAmount(total)}${hasPartialAmounts ? '+' : ''}` : '••••'}
                  </span>
                </div>
              )}
              {annualBillsWithAmount.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 1, alignItems: 'flex-end' }}>
                  <span style={{ fontSize: 9, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>Annual</span>
                  <span style={{ fontFamily: "'JetBrains Mono', monospace", fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
                    {amountsVisible ? `${formatAmount(annualTotal)}${hasPartialAnnual ? '+' : ''}` : '••••'}
                  </span>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </Card>
  );
}
