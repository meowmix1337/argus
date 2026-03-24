import React, { useState, useEffect, useRef, useCallback } from 'react';
import { BillsSettings } from './BillsSettings';

interface BillsPanelProps {
  onClose: () => void;
}

const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function BillsPanel({ onClose }: BillsPanelProps): React.ReactElement {
  const [isOpen, setIsOpen] = useState(false);

  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const prevFocusRef = useRef<HTMLElement | null>(null);
  const closingRef = useRef(false);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const handleClose = useCallback((): void => {
    if (closingRef.current) return;
    closingRef.current = true;
    setIsOpen(false);
    closeTimerRef.current = setTimeout(() => {
      prevFocusRef.current?.focus();
      onClose();
    }, 260);
  }, [onClose]);

  useEffect(() => {
    return () => clearTimeout(closeTimerRef.current);
  }, []);

  useEffect(() => {
    prevFocusRef.current = document.activeElement as HTMLElement;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => setIsOpen(true));
    });
  }, []);

  useEffect(() => {
    if (isOpen) closeBtnRef.current?.focus();
  }, [isOpen]);

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        handleClose();
        return;
      }
      if (e.key === 'Tab' && panelRef.current) {
        const focusable = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE));
        if (focusable.length === 0) return;
        const first = focusable[0];
        const last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [handleClose]);

  function handleBackdropClick(e: React.MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) handleClose();
  }

  return (
    <div
      onClick={handleBackdropClick}
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 1000,
        background: isOpen ? 'rgba(0,0,0,0.35)' : 'rgba(0,0,0,0)',
        transition: 'background 0.3s ease',
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-label="Manage Bills"
        aria-modal="true"
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          bottom: 0,
          width: 440,
          maxWidth: '100vw',
          background: 'rgba(20,20,35,0.95)',
          borderLeft: '1px solid rgba(255,255,255,0.1)',
          backdropFilter: 'blur(24px)',
          boxShadow: isOpen
            ? '-8px 0 40px rgba(0,0,0,0.5), -1px 0 0 rgba(99,102,241,0.15)'
            : 'none',
          transform: isOpen ? 'translateX(0)' : 'translateX(100%)',
          transition: isOpen
            ? 'transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.3s ease'
            : 'transform 0.24s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.24s ease',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '20px 24px',
          borderBottom: '1px solid rgba(255,255,255,0.08)',
          flexShrink: 0,
        }}>
          <div>
            <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--text-primary)' }}>
              Bills
            </span>
            <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>
              Manage your recurring bills and due dates
            </div>
          </div>
          <button
            ref={closeBtnRef}
            onClick={handleClose}
            style={{
              width: 32,
              height: 32,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'rgba(255,255,255,0.06)',
              border: '1px solid rgba(255,255,255,0.1)',
              borderRadius: 8,
              cursor: 'pointer',
              color: 'var(--text-secondary)',
              fontSize: 14,
            }}
            aria-label="Close bills panel"
          >
            ✕
          </button>
        </div>

        {/* Scrollable content */}
        <div style={{
          flex: 1,
          overflowY: 'auto',
          padding: 24,
        }}>
          <BillsSettings />
        </div>
      </div>
    </div>
  );
}
