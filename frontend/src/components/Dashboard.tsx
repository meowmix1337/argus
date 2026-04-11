import React, { useCallback, useEffect, useState } from 'react';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  TouchSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
  closestCenter,
} from '@dnd-kit/core';
import {
  SortableContext,
  arrayMove,
  rectSortingStrategy,
  sortableKeyboardCoordinates,
  useSortable,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { useDashboard } from '../hooks/useDashboard';
import { useClock } from '../hooks/useClock';
import { useAuth } from '../hooks/useAuth';
import { useTheme } from '../hooks/useTheme';
import { useWindowSize } from '../hooks/useWindowSize';
import { useCardOrder, type CardId } from '../hooks/useCardOrder';
import { WeatherCard } from './WeatherCard';
import { CalendarCard } from './CalendarCard';
import { TasksCard } from './TasksCard';
import { NewsCard } from './NewsCard';
import { StocksCard } from './StocksCard';
import { QuoteCard } from './QuoteCard';
import { BillsCard } from './BillsCard';
import { UserProfile } from './UserProfile';
import { UnavailableCard } from './ui/UnavailableCard';
import { SettingsPanel } from './SettingsPanel';
import { BillsPanel } from './BillsPanel';
import { IntegrationsPanel } from './IntegrationsPanel';
import { NotificationBell } from './NotificationBell';
import { NotificationPanel } from './NotificationPanel';
import { useUnreadCount } from '../hooks/useNotifications';
import { SocialSection } from './social/SocialSection';

function getGreeting(date: Date): string {
  const h = date.getHours();
  if (h < 12) return 'Good morning';
  if (h < 17) return 'Good afternoon';
  return 'Good evening';
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: true,
  });
}

function formatDate(date: Date): string {
  return date.toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  });
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

function CardSkeleton({ span = 1, rowSpan = 1, rows = 3 }: { span?: number; rowSpan?: number; rows?: number }): React.ReactElement {
  return (
    <div style={{
      gridColumn: `span ${span}`,
      ...(rowSpan > 1 ? { gridRow: `span ${rowSpan}` } : {}),
      height: '100%',
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 16,
      padding: 24,
    }}>
      <Skeleton height={14} width="40%" />
      <div style={{ marginTop: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
        {Array.from({ length: rows }).map((_, i) => (
          <Skeleton key={i} height={14} width={`${70 + (i % 3) * 10}%`} />
        ))}
      </div>
    </div>
  );
}

// Sortable wrapper — owns gridColumn, ghost slot, pill grip handle, and drop highlighting
interface SortableCardWrapperProps {
  id: CardId;
  span: number;
  rowSpan?: number;
  children: React.ReactNode;
}

function SortableCardWrapper({ id, span, rowSpan = 1, children }: SortableCardWrapperProps): React.ReactElement {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
    isOver,
  } = useSortable({ id });

  const [gripHovered, setGripHovered] = useState(false);

  // FROM location: invisible slot — card is floating via DragOverlay, source just holds space
  if (isDragging) {
    return (
      <div
        ref={setNodeRef}
        style={{ gridColumn: `span ${span}`, ...(rowSpan > 1 ? { gridRow: `span ${rowSpan}` } : {}), height: '100%', borderRadius: 16 }}
      >
        <div style={{ visibility: 'hidden', pointerEvents: 'none' }}>{children}</div>
      </div>
    );
  }

  // TO location: dashed ghost slot — shows exactly where the dragged card will land.
  // No transform/transition here — the drop zone must stay at the card's natural grid
  // position, not follow the sort-preview animation (which would shift it far off-screen
  // for multi-row cards like News).
  if (isOver) {
    return (
      <div
        ref={setNodeRef}
        style={{
          gridColumn: `span ${span}`,
          ...(rowSpan > 1 ? { gridRow: `span ${rowSpan}` } : {}),
          height: '100%',
          borderRadius: 16,
          border: '2px dashed rgba(99,102,241,0.55)',
          background: 'rgba(99,102,241,0.07)',
          boxShadow: '0 0 0 4px rgba(99,102,241,0.12)',
        }}
      >
        <div style={{ visibility: 'hidden', pointerEvents: 'none' }}>{children}</div>
      </div>
    );
  }

  return (
    <div
      ref={setNodeRef}
      style={{
        gridColumn: `span ${span}`,
        ...(rowSpan > 1 ? { gridRow: `span ${rowSpan}` } : {}),
        height: '100%',
        position: 'relative',
        transform: CSS.Transform.toString(transform) ?? undefined,
        transition,
        borderRadius: 16,
      }}
    >
      {/* Centered pill grip — large hit area, subtle pill visual that grows on hover */}
      <div
        {...attributes}
        {...listeners}
        onMouseEnter={() => setGripHovered(true)}
        onMouseLeave={() => setGripHovered(false)}
        style={{
          position: 'absolute',
          top: 0,
          left: '50%',
          transform: 'translateX(-50%)',
          width: 64,
          height: 22,
          cursor: 'grab',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          touchAction: 'none',
          userSelect: 'none',
          zIndex: 2,
        }}
      >
        <div style={{
          width: gripHovered ? 40 : 28,
          height: 4,
          borderRadius: 2,
          background: gripHovered ? 'rgba(99,102,241,0.65)' : 'rgba(255,255,255,0.18)',
          transition: 'background 0.2s ease, width 0.2s ease',
        }} />
      </div>

      {children}
    </div>
  );
}

// The floating card rendered by DragOverlay while a drag is in progress
function DragOverlayCard({ children }: { children: React.ReactNode }): React.ReactElement {
  return (
    <div style={{
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 16,
      padding: 24,
      backdropFilter: 'blur(20px)',
      boxShadow: '0 28px 60px rgba(0,0,0,0.55), 0 0 0 1px rgba(99,102,241,0.4)',
      scale: '1.03',
      opacity: 0.97,
      cursor: 'grabbing',
    }}>
      {children}
    </div>
  );
}

export default function Dashboard(): React.ReactElement {
  const { data, isLoading, isError, refetch } = useDashboard();
  const { user } = useAuth();
  const { theme, toggleTheme } = useTheme();
  const now = useClock();
  const { breakpoint } = useWindowSize();
  const [headerLoaded, setHeaderLoaded] = useState(false);
  const [toggleHovered, setToggleHovered] = useState(false);
  const [cardOrder, setCardOrder] = useCardOrder();
  const [activeId, setActiveId] = useState<CardId | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [showBills, setShowBills] = useState(false);
  const [showIntegrations, setShowIntegrations] = useState(false);
  const [showNotifications, setShowNotifications] = useState(false);
  const { data: unreadData } = useUnreadCount();

  const isMobile = breakpoint === 'mobile';
  const isTablet = breakpoint === 'tablet';

  useEffect(() => {
    const t = setTimeout(() => setHeaderLoaded(true), 100);
    return () => clearTimeout(t);
  }, []);

  const lastUpdated = formatTime(now);

  function getSpan(id: CardId): number {
    if (id === 'news') return isMobile ? 1 : 2;
    return 1;
  }

  function getRowSpan(id: CardId): number {
    if (id === 'news' && !isMobile && !isTablet) return 2;
    return 1;
  }

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 250, tolerance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  );

  function handleDragStart(event: DragStartEvent): void {
    setActiveId(event.active.id as CardId);
  }

  function handleDragEnd(event: DragEndEvent): void {
    setActiveId(null);
    const { active, over } = event;
    if (!over || active.id === over.id) return;
    const oldIndex = cardOrder.indexOf(active.id as CardId);
    const newIndex = cardOrder.indexOf(over.id as CardId);
    if (oldIndex === -1 || newIndex === -1) return;
    setCardOrder(arrayMove(cardOrder, oldIndex, newIndex));
  }

  function handleDragCancel(): void {
    setActiveId(null);
  }

  const renderCard = useCallback(function renderCard(id: CardId): React.ReactNode {
    switch (id) {
      case 'weather':
        return data?.weather ? (
          <WeatherCard data={data.weather} delay={0.2} noGridSpan />
        ) : (
          <UnavailableCard span={1} label="Weather unavailable" noGridSpan />
        );
      case 'calendar':
        return data?.calendar ? (
          <CalendarCard events={data.calendar} delay={0.3} noGridSpan />
        ) : (
          <CardSkeleton span={1} rows={5} />
        );
      case 'tasks':
        return data?.tasks ? (
          <TasksCard tasks={data.tasks} tasksTotal={data.tasksTotal} delay={0.4} noGridSpan />
        ) : (
          <CardSkeleton span={1} rows={5} />
        );
      case 'news':
        return <NewsCard delay={0.5} isMobile={isMobile} isTablet={isTablet} noGridSpan />;
      case 'quote':
        return data?.meta?.quote?.text ? (
          <QuoteCard data={data.meta.quote} delay={0.6} noGridSpan />
        ) : (
          <UnavailableCard span={1} label="Quote unavailable" noGridSpan />
        );
      case 'bills':
        return (
          <BillsCard delay={0.7} noGridSpan onManage={() => setShowBills(true)} />
        );
      default:
        return null;
    }
  }, [data, isMobile, isTablet]);

  return (
    <div style={{
      minHeight: '100vh',
      background: 'var(--bg-primary)',
      color: 'var(--text-primary)',
      fontFamily: "'DM Sans', 'Helvetica Neue', sans-serif",
      padding: isMobile ? 16 : isTablet ? 24 : 32,
      position: 'relative',
    }}>
      <div style={{
        position: 'fixed',
        top: 0, left: 0, right: 0, bottom: 0,
        pointerEvents: 'none',
        zIndex: 0,
        background: `
          radial-gradient(ellipse 600px 400px at 15% 20%, var(--ambient-indigo) 0%, transparent 70%),
          radial-gradient(ellipse 500px 500px at 85% 70%, var(--ambient-pink) 0%, transparent 70%),
          radial-gradient(ellipse 400px 300px at 50% 90%, var(--ambient-green) 0%, transparent 70%)
        `,
      }} />

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.4; }
        }
      `}</style>

      <div style={{ position: 'relative', zIndex: 1, maxWidth: 1440, margin: '0 auto' }}>
        {/* Header */}
        <div style={{
          opacity: headerLoaded ? 1 : 0,
          transform: headerLoaded ? 'translateY(0)' : 'translateY(12px)',
          transition: 'all 0.8s cubic-bezier(0.16, 1, 0.3, 1)',
          display: 'flex',
          flexDirection: isMobile ? 'column' : 'row',
          justifyContent: 'space-between',
          alignItems: isMobile ? 'flex-start' : 'flex-end',
          gap: isMobile ? 16 : 0,
          marginBottom: isMobile ? 24 : 40,
          borderBottom: '1px solid var(--header-border)',
          paddingBottom: isMobile ? 16 : 24,
          position: 'relative',
          zIndex: 10,
        }}>
          <div>
            <div style={{
              fontSize: isMobile ? 12 : 14,
              fontWeight: 500,
              color: 'var(--text-accent)',
              letterSpacing: '0.12em',
              textTransform: 'uppercase',
              marginBottom: 8,
            }}>
              {formatDate(now)}
            </div>
            <h1 style={{
              fontFamily: "'Playfair Display', Georgia, serif",
              fontSize: isMobile ? 28 : isTablet ? 34 : 42,
              fontWeight: 700,
              margin: 0,
              background: 'linear-gradient(135deg, var(--gradient-heading-start) 0%, var(--gradient-heading-end) 100%)',
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
            }}>
              {getGreeting(now)}
            </h1>
          </div>
          <div style={{ display: 'flex', alignItems: isMobile ? 'center' : 'flex-end', gap: 16, alignSelf: isMobile ? 'stretch' : 'auto', justifyContent: isMobile ? 'space-between' : 'flex-end' }}>
            <div style={{ textAlign: isMobile ? 'left' : 'right' }}>
              <div style={{
                fontFamily: "'JetBrains Mono', monospace",
                fontSize: isMobile ? 22 : isTablet ? 26 : 32,
                fontWeight: 500,
                color: 'var(--text-clock)',
                letterSpacing: '-0.02em',
              }}>
                {formatTime(now)}
              </div>
              <div style={{ fontSize: isMobile ? 11 : 13, color: 'var(--text-secondary)', marginTop: 4 }}>
                {data?.meta?.sunrise
                  ? `☀️ ${data.meta.sunrise} → 🌙 ${data.meta.sunset} · ${data.meta.daylight} daylight`
                  : '☀️ — → 🌙 — · — daylight'}
              </div>
            </div>
            <button
              onClick={toggleTheme}
              aria-label={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              onMouseEnter={() => setToggleHovered(true)}
              onMouseLeave={() => setToggleHovered(false)}
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 36,
                height: 36,
                borderRadius: 8,
                background: toggleHovered ? 'var(--toggle-hover-bg)' : 'var(--toggle-bg)',
                border: '1px solid var(--toggle-border)',
                color: 'var(--toggle-text)',
                cursor: 'pointer',
                fontSize: 16,
                flexShrink: 0,
                transition: 'background 0.2s ease',
              }}
            >
              {theme === 'dark' ? '☀️' : '🌙'}
            </button>
            <NotificationBell
              unreadCount={unreadData?.count ?? 0}
              onClick={() => setShowNotifications((v) => !v)}
              isOpen={showNotifications}
            />
            {user && <UserProfile user={user} onOpenSettings={() => setShowSettings(true)} onOpenIntegrations={() => setShowIntegrations(true)} />}
          </div>
        </div>

        {/* Error banner */}
        {isError && (
          <div style={{
            marginBottom: 20,
            padding: '12px 16px',
            borderRadius: 10,
            background: 'var(--error-bg)',
            border: '1px solid var(--error-border)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}>
            <span style={{ fontSize: 14, color: 'var(--error-text)' }}>
              Failed to load dashboard data
            </span>
            <button
              onClick={() => refetch()}
              style={{
                fontSize: 13,
                color: 'var(--error-text)',
                background: 'var(--error-button-bg)',
                border: '1px solid var(--error-button-border)',
                borderRadius: 6,
                padding: '4px 12px',
                cursor: 'pointer',
              }}
            >
              Retry
            </button>
          </div>
        )}

        {/* Responsive grid — social feed sits in the last column alongside the cards */}
        <div style={{
          display: 'grid',
          gridTemplateColumns: isMobile ? '1fr' : isTablet ? '1fr 1fr' : '1fr 1fr 1fr',
          gap: isMobile ? 12 : 20,
          alignItems: 'start',
        }}>
          {/* Cards area — spans 2 of 3 cols on desktop so social takes the third */}
          <div style={{ gridColumn: isMobile || isTablet ? undefined : '1 / 3' }}>
            <StocksCard stocks={data?.stocks ?? null} delay={0.1} />

            {/* Card grid — 2 cols on desktop, 1 col on tablet/mobile */}
            <div style={{
              display: 'grid',
              gridTemplateColumns: isMobile || isTablet ? '1fr' : '1fr 1fr',
              gridAutoRows: isMobile || isTablet ? 'auto' : '420px',
              gap: isMobile ? 12 : 20,
            }}>
          {isLoading ? (
            <>
              <CardSkeleton span={1} rows={4} />
              <CardSkeleton span={1} rows={5} />
              <CardSkeleton span={1} rows={5} />
              <CardSkeleton span={isMobile ? 1 : 2} rowSpan={isMobile || isTablet ? 1 : 2} rows={4} />
              <CardSkeleton span={1} rows={2} />
            </>
          ) : (
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragStart={handleDragStart}
              onDragEnd={handleDragEnd}
              onDragCancel={handleDragCancel}
            >
              <SortableContext items={cardOrder} strategy={rectSortingStrategy}>
                {cardOrder.map((id) => (
                  <SortableCardWrapper key={id} id={id} span={getSpan(id)} rowSpan={getRowSpan(id)}>
                    {renderCard(id)}
                  </SortableCardWrapper>
                ))}
              </SortableContext>

              {/* Floating card that follows the cursor during drag */}
              <DragOverlay>
                {activeId ? (
                  <DragOverlayCard>
                    {renderCard(activeId)}
                  </DragOverlayCard>
                ) : null}
              </DragOverlay>
            </DndContext>
          )}
            </div>
          </div>

          {/* Right column — social feed, sticky whenever it sits beside the cards */}
          <div style={!isMobile ? {
            position: 'sticky',
            top: 32,
            maxHeight: 'calc(100vh - 64px)',
            overflowY: 'auto',
          } : { marginTop: 16 }}>
            <SocialSection />
          </div>
        </div>

        {/* Footer */}
        <div style={{
          marginTop: isMobile ? 16 : 32,
          paddingTop: 20,
          borderTop: '1px solid var(--footer-border)',
          display: 'flex',
          flexDirection: isMobile ? 'column' : 'row',
          justifyContent: 'space-between',
          alignItems: isMobile ? 'flex-start' : 'center',
          gap: isMobile ? 4 : 0,
          opacity: 0.5,
        }}>
          <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
            Daily Dashboard · Powered by Open-Meteo, GNews, Finnhub, Google APIs
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>
            Last updated: {lastUpdated}
          </div>
        </div>
      </div>

      {showSettings && <SettingsPanel onClose={() => setShowSettings(false)} />}
      {showBills && <BillsPanel onClose={() => setShowBills(false)} />}
      {showIntegrations && <IntegrationsPanel onClose={() => setShowIntegrations(false)} />}
      {showNotifications && <NotificationPanel onClose={() => setShowNotifications(false)} />}
    </div>
  );
}
