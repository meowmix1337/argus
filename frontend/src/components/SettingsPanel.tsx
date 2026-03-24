import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useUserSettings, useNewsCategories, useSettingsMutations } from '../hooks/useUserSettings';

interface SettingsPanelProps {
  onClose: () => void;
}

const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function SettingsPanel({ onClose }: SettingsPanelProps): React.ReactElement {
  const { settings } = useUserSettings();
  const { data: categoriesData } = useNewsCategories();
  const { save, saveCategories } = useSettingsMutations();

  const [isOpen, setIsOpen] = useState(false);
  const [latitude, setLatitude] = useState<string>('');
  const [longitude, setLongitude] = useState<string>('');
  const [timezone, setTimezone] = useState<string>('');
  const [calendarIcsUrl, setCalendarIcsUrl] = useState<string>('');
  const [selectedCategoryIds, setSelectedCategoryIds] = useState<string[]>([]);
  const [focusedField, setFocusedField] = useState<string | null>(null);
  const [showIcsUrl, setShowIcsUrl] = useState(false);
  const [saved, setSaved] = useState(false);

  const closeBtnRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const prevFocusRef = useRef<HTMLElement | null>(null);
  // Safe: component is fully unmounted/remounted on each open, so closingRef resets
  const closingRef = useRef(false);
  const closeTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const savedTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const handleClose = useCallback((): void => {
    if (closingRef.current) return;
    closingRef.current = true;
    setIsOpen(false);
    closeTimerRef.current = setTimeout(() => {
      prevFocusRef.current?.focus();
      onClose();
    }, 260);
  }, [onClose]);

  // Clean up timers on unmount
  useEffect(() => {
    return () => {
      clearTimeout(closeTimerRef.current);
      clearTimeout(savedTimerRef.current);
    };
  }, []);

  // Animate in on mount
  useEffect(() => {
    prevFocusRef.current = document.activeElement as HTMLElement;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        setIsOpen(true);
      });
    });
  }, []);

  // Focus close button when panel opens
  useEffect(() => {
    if (isOpen) {
      closeBtnRef.current?.focus();
    }
  }, [isOpen]);

  // Escape to close + focus trap
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

  // Sync form state when server data arrives (React "adjust state during render" pattern)
  const [prevSettings, setPrevSettings] = useState(settings);
  if (settings && settings !== prevSettings) {
    setPrevSettings(settings);
    setLatitude(settings.latitude !== null ? String(settings.latitude) : '');
    setLongitude(settings.longitude !== null ? String(settings.longitude) : '');
    setTimezone(settings.timezone ?? '');
    setCalendarIcsUrl(settings.calendar_ics_url ?? '');
  }

  const [prevCategories, setPrevCategories] = useState(categoriesData);
  if (categoriesData && categoriesData !== prevCategories) {
    setPrevCategories(categoriesData);
    setSelectedCategoryIds(categoriesData.selected.map((c) => c.id));
  }

  // Auto-save for news categories
  function toggleCategory(id: string): void {
    const next = selectedCategoryIds.includes(id)
      ? selectedCategoryIds.filter((c) => c !== id)
      : [...selectedCategoryIds, id];
    setSelectedCategoryIds(next);
    saveCategories.mutate(next);
  }

  // Manual save for text inputs
  async function handleSaveSettings(): Promise<void> {
    const lat = latitude !== '' ? parseFloat(latitude) : null;
    const lon = longitude !== '' ? parseFloat(longitude) : null;
    const body = {
      latitude: lat !== null && !isNaN(lat) ? lat : null,
      longitude: lon !== null && !isNaN(lon) ? lon : null,
      timezone: timezone !== '' ? timezone : null,
      calendar_ics_url: calendarIcsUrl !== '' ? calendarIcsUrl : null,
    };
    try {
      await save.mutateAsync(body);
      setSaved(true);
      savedTimerRef.current = setTimeout(() => setSaved(false), 2000);
    } catch {
      // mutation handles its own error state via TanStack Query
    }
  }

  function handleBackdropClick(e: React.MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  }

  const inputStyle = (field: string): React.CSSProperties => ({
    background: 'rgba(255,255,255,0.06)',
    border: `1px solid ${focusedField === field ? 'rgba(99,102,241,0.5)' : 'rgba(255,255,255,0.1)'}`,
    borderRadius: 8,
    padding: '8px 12px',
    fontSize: 13,
    color: 'var(--text-primary)',
    outline: 'none',
    width: '100%',
    boxSizing: 'border-box' as const,
  });

  const sectionLabelStyle: React.CSSProperties = {
    fontSize: 12,
    fontWeight: 600,
    color: 'var(--text-secondary)',
    letterSpacing: '0.08em',
    textTransform: 'uppercase',
    marginBottom: 10,
  };

  const available = categoriesData?.available ?? [];

  // Staggered section fade-in
  const sectionStyle = (index: number): React.CSSProperties => ({
    opacity: isOpen ? 1 : 0,
    transform: isOpen ? 'translateY(0)' : 'translateY(8px)',
    transition: `opacity 0.3s ease ${index * 50 + 100}ms, transform 0.3s ease ${index * 50 + 100}ms`,
  });

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
        aria-label="Settings"
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
          <span style={{ fontSize: 18, fontWeight: 700, color: 'var(--text-primary)' }}>
            Settings
          </span>
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
            aria-label="Close settings"
          >
            ✕
          </button>
        </div>

        {/* Scrollable content */}
        <div style={{
          flex: 1,
          overflowY: 'auto',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 28,
        }}>
          {/* Section: Location */}
          <div style={sectionStyle(0)}>
            <div style={sectionLabelStyle}>Location</div>
            <div style={{ display: 'flex', gap: 12 }}>
              <div style={{ flex: 1 }}>
                <label htmlFor="settings-latitude" style={{ fontSize: 12, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>
                  Latitude
                </label>
                <input
                  id="settings-latitude"
                  type="number"
                  step={0.0001}
                  min={-90}
                  max={90}
                  value={latitude}
                  onChange={(e) => setLatitude(e.target.value)}
                  onFocus={() => setFocusedField('latitude')}
                  onBlur={() => setFocusedField(null)}
                  style={inputStyle('latitude')}
                  placeholder="37.7749"
                />
              </div>
              <div style={{ flex: 1 }}>
                <label htmlFor="settings-longitude" style={{ fontSize: 12, color: 'var(--text-muted)', display: 'block', marginBottom: 6 }}>
                  Longitude
                </label>
                <input
                  id="settings-longitude"
                  type="number"
                  step={0.0001}
                  min={-180}
                  max={180}
                  value={longitude}
                  onChange={(e) => setLongitude(e.target.value)}
                  onFocus={() => setFocusedField('longitude')}
                  onBlur={() => setFocusedField(null)}
                  style={inputStyle('longitude')}
                  placeholder="-122.4194"
                />
              </div>
            </div>
          </div>

          {/* Section: Timezone */}
          <div style={sectionStyle(1)}>
            <label htmlFor="settings-timezone" style={{ ...sectionLabelStyle, display: 'block' }}>Timezone</label>
            <input
              id="settings-timezone"
              type="text"
              value={timezone}
              onChange={(e) => setTimezone(e.target.value)}
              onFocus={() => setFocusedField('timezone')}
              onBlur={() => setFocusedField(null)}
              style={inputStyle('timezone')}
              placeholder="e.g. America/New_York"
            />
          </div>

          {/* Section: Calendar ICS URL */}
          <div style={sectionStyle(2)}>
            <label htmlFor="settings-calendar-url" style={{ ...sectionLabelStyle, display: 'block' }}>Calendar ICS URL</label>
            <div style={{ position: 'relative' }}>
              <input
                id="settings-calendar-url"
                type={showIcsUrl ? 'url' : 'password'}
                value={calendarIcsUrl}
                onChange={(e) => setCalendarIcsUrl(e.target.value)}
                onFocus={() => setFocusedField('calendarIcsUrl')}
                onBlur={() => setFocusedField(null)}
                style={{ ...inputStyle('calendarIcsUrl'), paddingRight: 40 }}
                placeholder="https://..."
              />
              <button
                type="button"
                onClick={() => setShowIcsUrl((v) => !v)}
                aria-label={showIcsUrl ? 'Hide URL' : 'Show URL'}
                style={{
                  position: 'absolute',
                  right: 10,
                  top: '50%',
                  transform: 'translateY(-50%)',
                  background: 'none',
                  border: 'none',
                  cursor: 'pointer',
                  color: 'var(--text-muted)',
                  fontSize: 12,
                  padding: 0,
                  lineHeight: 1,
                }}
              >
                {showIcsUrl ? '🙈' : '👁'}
              </button>
            </div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 6 }}>
              Your private Google/Outlook calendar feed URL
            </div>
          </div>

          {/* Save button for text fields */}
          <div style={{
            ...sectionStyle(3),
            display: 'flex',
            alignItems: 'center',
            gap: 12,
          }}>
            <button
              onClick={() => void handleSaveSettings()}
              disabled={save.isPending}
              style={{
                background: '#6366f1',
                border: 'none',
                borderRadius: 8,
                padding: '9px 24px',
                fontSize: 13,
                fontWeight: 600,
                color: '#fff',
                cursor: save.isPending ? 'not-allowed' : 'pointer',
                opacity: save.isPending ? 0.7 : 1,
                transition: 'opacity 0.15s ease',
              }}
            >
              {save.isPending ? 'Saving...' : 'Save'}
            </button>
            {saved && (
              <span style={{ fontSize: 12, color: '#34d399' }}>
                Saved
              </span>
            )}
          </div>

          {/* Divider */}
          <div style={{
            ...sectionStyle(4),
            height: 1,
            background: 'rgba(255,255,255,0.08)',
          }} />

          {/* Section: News Categories (auto-save) */}
          <div style={sectionStyle(5)}>
            <div style={{
              ...sectionLabelStyle,
              display: 'flex',
              alignItems: 'center',
              gap: 8,
            }}>
              News Categories
              <span style={{
                fontSize: 10,
                fontWeight: 400,
                color: 'var(--text-muted)',
                letterSpacing: '0.02em',
                textTransform: 'none' as const,
              }}>
                auto-saves
              </span>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {available.map((category) => {
                const isSelected = selectedCategoryIds.includes(category.id);
                return (
                  <button
                    key={category.id}
                    onClick={() => toggleCategory(category.id)}
                    aria-pressed={isSelected}
                    style={{
                      background: isSelected ? 'rgba(99,102,241,0.2)' : 'rgba(255,255,255,0.05)',
                      border: isSelected
                        ? '1px solid rgba(99,102,241,0.5)'
                        : '1px solid rgba(255,255,255,0.1)',
                      borderRadius: 8,
                      padding: '6px 12px',
                      fontSize: 12,
                      cursor: 'pointer',
                      color: isSelected ? 'var(--text-accent)' : 'var(--text-secondary)',
                      transition: 'all 0.15s ease',
                    }}
                  >
                    {category.label}
                  </button>
                );
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
