import React, { useState } from 'react';
import { useBills } from '../hooks/useBills';
import type { Bill } from '../types/dashboard';
import type { BillPayload } from '../api/client';

const CATEGORIES = [
  { id: 'rent',          label: 'Rent / Mortgage' },
  { id: 'utilities',     label: 'Utilities' },
  { id: 'subscriptions', label: 'Subscriptions' },
  { id: 'insurance',     label: 'Insurance' },
  { id: 'loans',         label: 'Loans' },
  { id: 'medical',       label: 'Medical' },
  { id: 'other',         label: 'Other' },
];

const RECURRENCE_TYPES = [
  { id: 'once',      label: 'One-time' },
  { id: 'weekly',    label: 'Weekly' },
  { id: 'biweekly',  label: 'Every two weeks' },
  { id: 'monthly',   label: 'Monthly' },
  { id: 'quarterly', label: 'Quarterly' },
  { id: 'annual',    label: 'Annual' },
];

const CATEGORY_COLORS: Record<string, string> = {
  rent: '#f97316',
  utilities: '#3b82f6',
  subscriptions: '#8b5cf6',
  insurance: '#ec4899',
  loans: '#ef4444',
  medical: '#10b981',
  other: '#6b7280',
};

function recurrenceSummary(bill: Bill): string {
  switch (bill.recurrenceType) {
    case 'once':      return bill.dueDate ?? '';
    case 'monthly':   return bill.dueDay != null ? `Day ${bill.dueDay}` : 'Monthly';
    case 'annual':    return bill.dueMonth != null && bill.dueDay != null
      ? `${['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'][bill.dueMonth - 1]} ${bill.dueDay}`
      : 'Annual';
    case 'weekly':    return 'Weekly';
    case 'biweekly':  return 'Biweekly';
    case 'quarterly': return 'Quarterly';
    default: return bill.recurrenceType;
  }
}

interface BillFormState {
  name: string;
  amount: string;
  categoryId: string;
  recurrenceType: Bill['recurrenceType'];
  dueDate: string;
  dueDay: string;
  dueMonth: string;
  anchorDate: string;
  notes: string;
}

const EMPTY_FORM: BillFormState = {
  name: '',
  amount: '',
  categoryId: 'other',
  recurrenceType: 'monthly',
  dueDate: '',
  dueDay: '',
  dueMonth: '',
  anchorDate: '',
  notes: '',
};

function billToForm(bill: Bill): BillFormState {
  return {
    name: bill.name,
    amount: bill.amount != null ? String(bill.amount) : '',
    categoryId: bill.categoryId,
    recurrenceType: bill.recurrenceType,
    dueDate: bill.dueDate ?? '',
    dueDay: bill.dueDay != null ? String(bill.dueDay) : '',
    dueMonth: bill.dueMonth != null ? String(bill.dueMonth) : '',
    anchorDate: bill.anchorDate ?? '',
    notes: bill.notes ?? '',
  };
}

function formToPayload(form: BillFormState): BillPayload {
  const amount = form.amount !== '' ? parseFloat(form.amount) : null;
  const dueDay = form.dueDay !== '' ? parseInt(form.dueDay) : null;
  const dueMonth = form.dueMonth !== '' ? parseInt(form.dueMonth) : null;
  return {
    name: form.name.trim(),
    amount: amount !== null && !isNaN(amount) ? amount : null,
    categoryId: form.categoryId,
    recurrenceType: form.recurrenceType,
    dueDate: form.recurrenceType === 'once' ? form.dueDate || null : null,
    dueDay: (form.recurrenceType === 'monthly' || form.recurrenceType === 'annual') ? dueDay : null,
    dueMonth: form.recurrenceType === 'annual' ? dueMonth : null,
    anchorDate: ['weekly', 'biweekly', 'quarterly'].includes(form.recurrenceType) ? form.anchorDate || null : null,
    notes: form.notes.trim() || null,
  };
}

const inputStyle: React.CSSProperties = {
  background: 'rgba(255,255,255,0.06)',
  border: '1px solid rgba(255,255,255,0.1)',
  borderRadius: 8,
  padding: '7px 10px',
  fontSize: 12,
  color: 'var(--text-primary)',
  outline: 'none',
  width: '100%',
  boxSizing: 'border-box',
};

const labelStyle: React.CSSProperties = {
  fontSize: 11,
  color: 'var(--text-muted)',
  display: 'block',
  marginBottom: 4,
};

interface BillFormProps {
  initial: BillFormState;
  isPending: boolean;
  onSave: (payload: BillPayload) => void;
  onCancel: () => void;
}

function BillForm({ initial, isPending, onSave, onCancel }: BillFormProps): React.ReactElement {
  const [form, setForm] = useState<BillFormState>(initial);
  const set = (field: keyof BillFormState, value: string) =>
    setForm((f) => ({ ...f, [field]: value }));

  function handleSubmit(e: React.FormEvent): void {
    e.preventDefault();
    onSave(formToPayload(form));
  }

  return (
    <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {/* Name + Amount row */}
      <div style={{ display: 'flex', gap: 8 }}>
        <div style={{ flex: 2 }}>
          <label style={labelStyle}>Name *</label>
          <input
            style={inputStyle}
            value={form.name}
            onChange={(e) => set('name', e.target.value)}
            placeholder="e.g. Netflix"
            required
            maxLength={200}
          />
        </div>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Amount</label>
          <input
            style={inputStyle}
            type="number"
            step="0.01"
            min="0.01"
            value={form.amount}
            onChange={(e) => set('amount', e.target.value)}
            placeholder="Optional"
          />
        </div>
      </div>

      {/* Category + Recurrence row */}
      <div style={{ display: 'flex', gap: 8 }}>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Category *</label>
          <select
            style={{ ...inputStyle, cursor: 'pointer' }}
            value={form.categoryId}
            onChange={(e) => set('categoryId', e.target.value)}
          >
            {CATEGORIES.map((c) => (
              <option key={c.id} value={c.id}>{c.label}</option>
            ))}
          </select>
        </div>
        <div style={{ flex: 1 }}>
          <label style={labelStyle}>Recurrence *</label>
          <select
            style={{ ...inputStyle, cursor: 'pointer' }}
            value={form.recurrenceType}
            onChange={(e) => set('recurrenceType', e.target.value as Bill['recurrenceType'])}
          >
            {RECURRENCE_TYPES.map((r) => (
              <option key={r.id} value={r.id}>{r.label}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Conditional schedule fields */}
      {form.recurrenceType === 'once' && (
        <div>
          <label style={labelStyle}>Due Date *</label>
          <input
            style={inputStyle}
            type="date"
            value={form.dueDate}
            onChange={(e) => set('dueDate', e.target.value)}
            required
          />
        </div>
      )}

      {form.recurrenceType === 'monthly' && (
        <div>
          <label style={labelStyle}>Day of month (1–31) *</label>
          <input
            style={inputStyle}
            type="number"
            min={1}
            max={31}
            value={form.dueDay}
            onChange={(e) => set('dueDay', e.target.value)}
            placeholder="e.g. 15"
            required
          />
        </div>
      )}

      {form.recurrenceType === 'annual' && (
        <div style={{ display: 'flex', gap: 8 }}>
          <div style={{ flex: 1 }}>
            <label style={labelStyle}>Month (1–12) *</label>
            <input
              style={inputStyle}
              type="number"
              min={1}
              max={12}
              value={form.dueMonth}
              onChange={(e) => set('dueMonth', e.target.value)}
              placeholder="e.g. 4"
              required
            />
          </div>
          <div style={{ flex: 1 }}>
            <label style={labelStyle}>Day (1–31) *</label>
            <input
              style={inputStyle}
              type="number"
              min={1}
              max={31}
              value={form.dueDay}
              onChange={(e) => set('dueDay', e.target.value)}
              placeholder="e.g. 15"
              required
            />
          </div>
        </div>
      )}

      {['weekly', 'biweekly', 'quarterly'].includes(form.recurrenceType) && (
        <div>
          <label style={labelStyle}>Anchor date *</label>
          <input
            style={inputStyle}
            type="date"
            value={form.anchorDate}
            onChange={(e) => set('anchorDate', e.target.value)}
            required
          />
          <div style={{ fontSize: 10, color: 'var(--text-muted)', marginTop: 3 }}>
            First occurrence — future recurrences are computed from this date
          </div>
        </div>
      )}

      {/* Notes */}
      <div>
        <label style={labelStyle}>Notes</label>
        <input
          style={inputStyle}
          value={form.notes}
          onChange={(e) => set('notes', e.target.value)}
          placeholder="Optional note or URL"
          maxLength={500}
        />
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
        <button
          type="button"
          onClick={onCancel}
          style={{
            background: 'rgba(255,255,255,0.06)',
            border: '1px solid rgba(255,255,255,0.1)',
            borderRadius: 7,
            padding: '6px 16px',
            fontSize: 12,
            color: 'var(--text-secondary)',
            cursor: 'pointer',
          }}
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={isPending}
          style={{
            background: '#6366f1',
            border: 'none',
            borderRadius: 7,
            padding: '6px 16px',
            fontSize: 12,
            fontWeight: 600,
            color: '#fff',
            cursor: isPending ? 'not-allowed' : 'pointer',
            opacity: isPending ? 0.7 : 1,
          }}
        >
          {isPending ? 'Saving…' : 'Save'}
        </button>
      </div>
    </form>
  );
}

export function BillsSettings(): React.ReactElement {
  const { bills, create, update, remove } = useBills();
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  function handleCreate(payload: BillPayload): void {
    create.mutate(payload, { onSuccess: () => setAdding(false) });
  }

  function handleUpdate(id: string, payload: BillPayload): void {
    update.mutate({ id, ...payload }, { onSuccess: () => setEditingId(null) });
  }

  function handleDelete(id: string): void {
    remove.mutate(id);
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      {/* Existing bills list */}
      {bills.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {bills.map((bill) => {
            const color = CATEGORY_COLORS[bill.categoryId] ?? CATEGORY_COLORS.other;
            const isEditing = editingId === bill.id;

            return (
              <div key={bill.id} style={{
                borderRadius: 10,
                border: '1px solid rgba(255,255,255,0.08)',
                overflow: 'hidden',
              }}>
                {isEditing ? (
                  <div style={{ padding: '12px' }}>
                    <BillForm
                      initial={billToForm(bill)}
                      isPending={update.isPending}
                      onSave={(payload) => handleUpdate(bill.id, payload)}
                      onCancel={() => setEditingId(null)}
                    />
                  </div>
                ) : (
                  <div style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    padding: '10px 12px',
                  }}>
                    <div style={{
                      width: 8,
                      height: 8,
                      borderRadius: '50%',
                      background: color,
                      flexShrink: 0,
                    }} />
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
                        {recurrenceSummary(bill)}
                        {bill.amount != null ? ` · $${bill.amount}` : ''}
                      </div>
                    </div>
                    <button
                      onClick={() => setEditingId(bill.id)}
                      style={{
                        background: 'none',
                        border: '1px solid rgba(255,255,255,0.1)',
                        borderRadius: 6,
                        padding: '3px 10px',
                        fontSize: 11,
                        color: 'var(--text-secondary)',
                        cursor: 'pointer',
                        flexShrink: 0,
                      }}
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(bill.id)}
                      disabled={remove.isPending}
                      style={{
                        background: 'none',
                        border: '1px solid rgba(239,68,68,0.2)',
                        borderRadius: 6,
                        padding: '3px 10px',
                        fontSize: 11,
                        color: '#ef4444',
                        cursor: remove.isPending ? 'not-allowed' : 'pointer',
                        flexShrink: 0,
                        opacity: remove.isPending ? 0.6 : 1,
                      }}
                    >
                      Delete
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Add form */}
      {adding ? (
        <div style={{
          borderRadius: 10,
          border: '1px solid rgba(99,102,241,0.25)',
          background: 'rgba(99,102,241,0.05)',
          padding: '12px',
        }}>
          <BillForm
            initial={EMPTY_FORM}
            isPending={create.isPending}
            onSave={handleCreate}
            onCancel={() => setAdding(false)}
          />
        </div>
      ) : (
        <button
          onClick={() => setAdding(true)}
          style={{
            background: 'rgba(99,102,241,0.1)',
            border: '1px dashed rgba(99,102,241,0.3)',
            borderRadius: 10,
            padding: '9px 0',
            fontSize: 12,
            color: 'var(--text-accent)',
            cursor: 'pointer',
            width: '100%',
            transition: 'background 0.15s',
          }}
        >
          + Add Bill
        </button>
      )}
    </div>
  );
}
