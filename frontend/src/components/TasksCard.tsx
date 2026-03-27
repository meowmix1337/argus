import React, { useEffect, useMemo, useRef, useState } from 'react';
import type { Task } from '../types/dashboard';
import { Card } from './ui/Card';
import { CardHeader } from './ui/CardHeader';
import { useTasks } from '../hooks/useTasks';
import { fetchTasksPage } from '../api/client';
import { useLabels, useLabelMutations, useTaskLabels } from '../hooks/useLabels';

interface TasksCardProps {
  tasks: Task[];
  tasksTotal?: number;
  delay?: number;
  noGridSpan?: boolean;
}

function sanitizeColor(c: string): string {
  return /^#[0-9a-fA-F]{3,8}$/.test(c) ? c : '#6366f1';
}

const PRIORITY_COLOR: Record<string, string> = {
  high: '#ef4444',
  medium: '#f59e0b',
  low: '#6b7280',
};

const PRIORITY_CYCLE: Task['priority'][] = ['high', 'medium', 'low'];

const COLOR_PRESETS = [
  '#6366f1',
  '#ec4899',
  '#f59e0b',
  '#10b981',
  '#3b82f6',
  '#ef4444',
  '#8b5cf6',
  '#06b6d4',
];

export function TasksCard({ tasks, tasksTotal, delay = 0, noGridSpan = false }: TasksCardProps): React.ReactElement {
  const { toggle: toggleMutation, create, remove: removeMutation } = useTasks();
  const { labels } = useLabels();
  const { createLabel, deleteLabel, assignLabel, removeLabel } = useLabelMutations();

  // Infinite scroll state
  const [extraTasks, setExtraTasks] = useState<Task[]>([]);

  // Wrap toggle/remove to also update extraTasks, which lives outside React Query
  const toggle = {
    ...toggleMutation,
    mutate: (vars: { id: string; done: boolean }) => {
      toggleMutation.mutate(vars);
      setExtraTasks((prev) => prev.map((t) => (t.id === vars.id ? { ...t, done: vars.done } : t)));
    },
  };
  const remove = {
    ...removeMutation,
    mutate: (id: string) => {
      removeMutation.mutate(id);
      setExtraTasks((prev) => prev.filter((t) => t.id !== id));
    },
  };
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const observerRef = useRef<IntersectionObserver | null>(null);
  const isLoadingMoreRef = useRef(false);

  // Merge first-page tasks with extra pages, deduplicating by ID
  const allTasks = useMemo(() => {
    const seenIds = new Set(tasks.map((t) => t.id));
    return [...tasks, ...extraTasks.filter((t) => !seenIds.has(t.id))];
  }, [tasks, extraTasks]);

  const totalCount = tasksTotal ?? tasks.length;
  const hasMore = allTasks.length < totalCount;

  // loadMore kept in a ref so the IntersectionObserver callback never goes stale
  const loadMoreRef = useRef<() => void>(() => undefined);
  // eslint-disable-next-line react-hooks/refs -- intentional render-time ref update to keep callback fresh
  loadMoreRef.current = () => {
    if (isLoadingMoreRef.current || !hasMore) return;
    isLoadingMoreRef.current = true;
    setIsLoadingMore(true);
    fetchTasksPage(5, allTasks.length)
      .then((result) => {
        setExtraTasks((prev) => [...prev, ...result.tasks]);
      })
      .catch(() => undefined)
      .finally(() => {
        isLoadingMoreRef.current = false;
        setIsLoadingMore(false);
        // Re-register the sentinel so the observer fires again if it's still visible
        const el = sentinelRef.current;
        if (el && observerRef.current) {
          observerRef.current.unobserve(el);
          observerRef.current.observe(el);
        }
      });
  };

  // Observe sentinel at the bottom of the task list.
  // root is the scroll container so intersection is measured relative to the
  // clipped viewport of that div, not the browser viewport.
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => { if (entries[0].isIntersecting) loadMoreRef.current(); },
      { root: scrollContainerRef.current, threshold: 0 },
    );
    observerRef.current = observer;
    observer.observe(el);
    return () => {
      observer.disconnect();
      observerRef.current = null;
    };
  }, []);

  const taskIds = useMemo(() => allTasks.map((t) => t.id), [allTasks]);
  const taskLabelsMap = useTaskLabels(taskIds);

  const [newText, setNewText] = useState('');
  const [newPriority, setNewPriority] = useState<Task['priority']>('medium');
  const [hoveredTaskId, setHoveredTaskId] = useState<string | null>(null);
  const [activeFilterLabel, setActiveFilterLabel] = useState<string | null>(null);
  const [labelPickerTaskId, setLabelPickerTaskId] = useState<string | null>(null);
  const [showLabelManager, setShowLabelManager] = useState(false);
  const [newLabelName, setNewLabelName] = useState('');
  const [newLabelColor, setNewLabelColor] = useState('#6366f1');

  const pickerRef = useRef<HTMLDivElement>(null);

  // Close label picker on outside click
  useEffect(() => {
    if (!labelPickerTaskId) return;
    function handleClick(e: MouseEvent) {
      if (pickerRef.current && !pickerRef.current.contains(e.target as Node)) {
        setLabelPickerTaskId(null);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [labelPickerTaskId]);

  function cyclePriority() {
    const idx = PRIORITY_CYCLE.indexOf(newPriority);
    setNewPriority(PRIORITY_CYCLE[(idx + 1) % PRIORITY_CYCLE.length]);
  }

  function handleAdd() {
    const text = newText.trim();
    if (!text) return;
    create.mutate({ text, priority: newPriority });
    setNewText('');
  }

  function handleAddLabel() {
    const name = newLabelName.trim();
    if (!name) return;
    createLabel.mutate({ name, color: newLabelColor });
    setNewLabelName('');
  }

  // Filter tasks by active label
  const visibleTasks = activeFilterLabel
    ? allTasks.filter((t) => {
        const tLabels = taskLabelsMap.get(t.id) ?? [];
        return tLabels.some((l) => l.id === activeFilterLabel);
      })
    : allTasks;

  const doneCount = allTasks.filter((t) => t.done).length;

  return (
    <Card delay={delay} noGridSpan={noGridSpan}>
      <CardHeader
        icon="◉"
        title="Tasks"
        badge={`${doneCount}/${totalCount} done`}
      />

      {/* Label filter bar */}
      {labels.length > 0 && (
        <div style={{
          display: 'flex',
          gap: 6,
          overflowX: 'auto',
          marginBottom: 10,
          paddingBottom: 2,
          scrollbarWidth: 'none',
        }}>
          {/* "All" chip */}
          <button
            onClick={() => setActiveFilterLabel(null)}
            style={{
              background: activeFilterLabel === null ? 'rgba(99,102,241,0.2)' : 'rgba(99,102,241,0.08)',
              border: activeFilterLabel === null ? '1px solid #6366f1' : '1px solid rgba(99,102,241,0.3)',
              color: '#6366f1',
              borderRadius: 999,
              padding: '2px 9px',
              fontSize: 11,
              fontWeight: 600,
              cursor: 'pointer',
              whiteSpace: 'nowrap',
            }}
          >
            All
          </button>
          {labels.map((label) => {
            const isActive = activeFilterLabel === label.id;
            return (
              <button
                key={label.id}
                onClick={() => setActiveFilterLabel(isActive ? null : label.id)}
                style={{
                  background: isActive ? `${sanitizeColor(label.color)}44` : `${sanitizeColor(label.color)}22`,
                  border: isActive ? `1px solid ${sanitizeColor(label.color)}` : `1px solid ${sanitizeColor(label.color)}66`,
                  color: sanitizeColor(label.color),
                  borderRadius: 999,
                  padding: '2px 9px',
                  fontSize: 11,
                  fontWeight: 600,
                  cursor: 'pointer',
                  whiteSpace: 'nowrap',
                }}
              >
                {label.name}
              </button>
            );
          })}
        </div>
      )}

      {/* Scrollable task list — capped height so the card never grows unbounded */}
      <div
        ref={scrollContainerRef}
        style={{
          maxHeight: 'min(320px, 40vh)',
          overflowY: 'auto',
          overflowX: 'hidden',
          // Thin scrollbar: WebKit thumb blends with the glass-morphism palette
          scrollbarWidth: 'thin',
          scrollbarColor: 'rgba(99,102,241,0.25) transparent',
          // Bottom fade mask signals there is more content below
          WebkitMaskImage: 'linear-gradient(to bottom, black calc(100% - 24px), transparent 100%)',
          maskImage: 'linear-gradient(to bottom, black calc(100% - 24px), transparent 100%)',
          paddingBottom: 8,
          marginBottom: -4,
        }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
        {visibleTasks.map((task) => {
          const tLabels = taskLabelsMap.get(task.id) ?? [];
          const isPickerOpen = labelPickerTaskId === task.id;

          return (
            <div
              key={task.id}
              onMouseEnter={() => setHoveredTaskId(task.id)}
              onMouseLeave={() => setHoveredTaskId(null)}
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 12,
                padding: '10px 12px',
                borderRadius: 10,
                background: task.done
                  ? 'rgba(16,185,129,0.05)'
                  : 'var(--bg-card)',
                transition: 'all 0.2s',
                position: 'relative',
                zIndex: isPickerOpen ? 1 : undefined,
              }}
            >
              {/* Checkbox */}
              <div
                onClick={() => toggle.mutate({ id: task.id, done: !task.done })}
                style={{
                  width: 20,
                  height: 20,
                  borderRadius: 6,
                  flexShrink: 0,
                  border: task.done ? '2px solid #10b981' : '2px solid var(--border-medium)',
                  background: task.done ? '#10b981' : 'transparent',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: 'pointer',
                  transition: 'all 0.2s',
                  fontSize: 12,
                  color: 'var(--bg-primary)',
                  marginTop: 1,
                }}
              >
                {task.done && '✓'}
              </div>

              {/* Text + labels */}
              <div style={{ flex: 1, minWidth: 0 }}>
                <div
                  onClick={() => toggle.mutate({ id: task.id, done: !task.done })}
                  style={{
                    fontSize: 13,
                    color: task.done ? 'var(--text-muted)' : 'var(--text-primary)',
                    textDecoration: task.done ? 'line-through' : 'none',
                    cursor: 'pointer',
                    transition: 'all 0.2s',
                  }}
                >
                  {task.text}
                </div>

                {/* Label chips on task */}
                {tLabels.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4 }}>
                    {tLabels.map((label) => (
                      <button
                        key={label.id}
                        onClick={() => removeLabel.mutate({ taskId: task.id, labelId: label.id })}
                        title="Remove label"
                        style={{
                          background: `${sanitizeColor(label.color)}22`,
                          border: `1px solid ${sanitizeColor(label.color)}66`,
                          color: sanitizeColor(label.color),
                          borderRadius: 999,
                          padding: '1px 7px',
                          fontSize: 10,
                          fontWeight: 600,
                          cursor: 'pointer',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {label.name}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              {/* Priority dot */}
              <div style={{
                width: 7,
                height: 7,
                borderRadius: '50%',
                flexShrink: 0,
                background: PRIORITY_COLOR[task.priority] ?? 'var(--text-secondary)',
                opacity: task.done ? 0.3 : 1,
                marginTop: 7,
              }} />

              {/* Label picker button */}
              <div style={{ position: 'relative', flexShrink: 0 }} ref={isPickerOpen ? pickerRef : undefined}>
                <button
                  onClick={() => setLabelPickerTaskId(isPickerOpen ? null : task.id)}
                  title="Assign label"
                  style={{
                    background: 'none',
                    border: 'none',
                    padding: '0 2px',
                    cursor: 'pointer',
                    fontSize: 13,
                    lineHeight: 1,
                    color: 'var(--text-secondary)',
                    opacity: hoveredTaskId === task.id || isPickerOpen ? 0.7 : 0,
                    transition: 'opacity 0.15s',
                  }}
                >
                  🏷
                </button>

                {/* Label picker dropdown */}
                {isPickerOpen && (
                  <div
                    onMouseDown={(e) => e.stopPropagation()}
                    style={{
                    position: 'absolute',
                    right: 0,
                    top: '100%',
                    zIndex: 100,
                    background: 'var(--bg-card)',
                    border: '1px solid var(--border-medium)',
                    borderRadius: 10,
                    padding: 8,
                    minWidth: 160,
                    boxShadow: '0 8px 24px rgba(0,0,0,0.3)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 4,
                  }}>
                    {labels.length === 0 && (
                      <div style={{ fontSize: 11, color: 'var(--text-muted)', padding: '2px 4px' }}>
                        No labels yet
                      </div>
                    )}
                    {labels.map((label) => {
                      const assigned = tLabels.some((l) => l.id === label.id);
                      return (
                        <button
                          key={label.id}
                          onClick={() => {
                            if (assigned) {
                              removeLabel.mutate({ taskId: task.id, labelId: label.id });
                            } else {
                              assignLabel.mutate({ taskId: task.id, labelId: label.id });
                            }
                          }}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 8,
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            padding: '4px 6px',
                            borderRadius: 6,
                            textAlign: 'left',
                            width: '100%',
                          }}
                        >
                          <span style={{
                            width: 10,
                            height: 10,
                            borderRadius: '50%',
                            background: sanitizeColor(label.color),
                            flexShrink: 0,
                          }} />
                          <span style={{ flex: 1, fontSize: 12, color: 'var(--text-primary)' }}>
                            {label.name}
                          </span>
                          {assigned && (
                            <span style={{ fontSize: 11, color: '#10b981' }}>✓</span>
                          )}
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>

              {/* Delete button */}
              <button
                onClick={() => remove.mutate(task.id)}
                style={{
                  background: 'none',
                  border: 'none',
                  padding: '0 2px',
                  cursor: 'pointer',
                  fontSize: 14,
                  lineHeight: 1,
                  color: 'var(--text-secondary)',
                  opacity: hoveredTaskId === task.id ? 0.7 : 0,
                  transition: 'opacity 0.15s',
                  flexShrink: 0,
                }}
                title="Delete task"
              >
                ×
              </button>
            </div>
          );
        })}
        </div>

        {/* Infinite scroll sentinel — lives inside the scroll container so the
            IntersectionObserver (root = scroll container) fires correctly */}
        <div ref={sentinelRef} style={{ paddingTop: 4 }}>
          {isLoadingMore && (
            <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-muted)', padding: '4px 0' }}>
              Loading more...
            </div>
          )}
        </div>
      </div>{/* end scroll container */}

      {/* Add task form */}
      <div style={{
        marginTop: 12,
        paddingTop: 12,
        borderTop: '1px solid var(--border-subtle)',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
      }}>
        {/* Priority selector */}
        <button
          onClick={cyclePriority}
          title={`Priority: ${newPriority}`}
          style={{
            width: 20,
            height: 20,
            borderRadius: '50%',
            flexShrink: 0,
            background: PRIORITY_COLOR[newPriority],
            border: 'none',
            cursor: 'pointer',
            padding: 0,
          }}
        />

        {/* Text input */}
        <input
          type="text"
          value={newText}
          onChange={(e) => setNewText(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          placeholder="Add a task..."
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            fontSize: 13,
            color: 'var(--text-primary)',
            caretColor: 'var(--text-accent)',
          }}
        />

        {/* Submit button — only visible when input has text */}
        {newText.trim() && (
          <button
            onClick={handleAdd}
            style={{
              background: 'rgba(99,102,241,0.15)',
              border: '1px solid rgba(99,102,241,0.3)',
              borderRadius: 6,
              padding: '3px 10px',
              fontSize: 12,
              color: 'var(--text-accent)',
              cursor: 'pointer',
              flexShrink: 0,
            }}
          >
            Add
          </button>
        )}
      </div>

      {/* Label manager section */}
      <div style={{ marginTop: 12 }}>
        <button
          onClick={() => setShowLabelManager((v) => !v)}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            fontSize: 11,
            color: 'var(--text-muted)',
            padding: '2px 0',
            display: 'flex',
            alignItems: 'center',
            gap: 4,
          }}
        >
          <span style={{ fontSize: 10, transform: showLabelManager ? 'rotate(90deg)' : 'none', display: 'inline-block', transition: 'transform 0.15s' }}>▶</span>
          Labels {labels.length > 0 ? `(${labels.length})` : ''}
        </button>

        {showLabelManager && (
          <div style={{
            marginTop: 8,
            paddingTop: 8,
            borderTop: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            gap: 8,
          }}>
            {/* Existing labels */}
            {labels.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                {labels.map((label) => (
                  <div
                    key={label.id}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 4,
                      background: `${sanitizeColor(label.color)}22`,
                      border: `1px solid ${sanitizeColor(label.color)}66`,
                      borderRadius: 999,
                      padding: '2px 6px 2px 9px',
                    }}
                  >
                    <span style={{ fontSize: 11, fontWeight: 600, color: sanitizeColor(label.color) }}>
                      {label.name}
                    </span>
                    <button
                      onClick={() => deleteLabel.mutate(label.id)}
                      title="Delete label"
                      style={{
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        fontSize: 12,
                        lineHeight: 1,
                        color: sanitizeColor(label.color),
                        padding: 0,
                        opacity: 0.6,
                      }}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}

            {/* Create new label */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <input
                type="text"
                value={newLabelName}
                onChange={(e) => setNewLabelName(e.target.value.slice(0, 16))}
                onKeyDown={(e) => e.key === 'Enter' && handleAddLabel()}
                placeholder="Label name..."
                maxLength={16}
                style={{
                  background: 'transparent',
                  border: 'none',
                  outline: 'none',
                  fontSize: 12,
                  color: 'var(--text-primary)',
                  caretColor: 'var(--text-accent)',
                  borderBottom: '1px solid var(--border-subtle)',
                  paddingBottom: 4,
                }}
              />
              <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                {COLOR_PRESETS.map((color) => (
                  <button
                    key={color}
                    onClick={() => setNewLabelColor(color)}
                    style={{
                      width: 18,
                      height: 18,
                      borderRadius: '50%',
                      background: color,
                      border: newLabelColor === color ? '2px solid var(--text-primary)' : '2px solid transparent',
                      cursor: 'pointer',
                      padding: 0,
                      flexShrink: 0,
                    }}
                  />
                ))}
                {newLabelName.trim() && (
                  <button
                    onClick={handleAddLabel}
                    style={{
                      marginLeft: 'auto',
                      background: 'rgba(99,102,241,0.15)',
                      border: '1px solid rgba(99,102,241,0.3)',
                      borderRadius: 6,
                      padding: '2px 8px',
                      fontSize: 11,
                      color: 'var(--text-accent)',
                      cursor: 'pointer',
                      flexShrink: 0,
                    }}
                  >
                    Add
                  </button>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}
