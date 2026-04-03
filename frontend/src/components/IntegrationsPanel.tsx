import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useGitHubIntegration, useGitHubRepos, useGitHubMutations } from '../hooks/useGitHubIntegration';
import { GitHubIcon } from './ui/GitHubIcon';

interface IntegrationsPanelProps {
  onClose: () => void;
}

const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function IntegrationsPanel({ onClose }: IntegrationsPanelProps): React.ReactElement {
  const { data: githubStatus } = useGitHubIntegration();
  const { data: githubRepos, isLoading: reposLoading } = useGitHubRepos(githubStatus?.connected ?? false);
  const { disconnect, updateWatchedRepos } = useGitHubMutations();

  const [showRepoPicker, setShowRepoPicker] = useState(false);
  const [selectedRepos, setSelectedRepos] = useState<string[]>(
    () => githubRepos?.filter((r) => r.watched).map((r) => r.fullName) ?? []
  );
  const [reposSaved, setReposSaved] = useState(false);
  const reposSavedTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

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
    return () => {
      clearTimeout(closeTimerRef.current);
      clearTimeout(reposSavedTimerRef.current);
    };
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

  // Sync selectedRepos when data changes after mount
  const [prevRepos, setPrevRepos] = useState(githubRepos);
  if (githubRepos && githubRepos !== prevRepos) {
    setPrevRepos(githubRepos);
    setSelectedRepos(githubRepos.filter((r) => r.watched).map((r) => r.fullName));
  }

  function handleBackdropClick(e: React.MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) handleClose();
  }

  const sectionLabelStyle: React.CSSProperties = {
    fontSize: 12,
    fontWeight: 600,
    color: 'var(--text-secondary)',
    letterSpacing: '0.08em',
    textTransform: 'uppercase',
    marginBottom: 10,
  };

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
        aria-label="Integrations"
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
            Integrations
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
            aria-label="Close integrations"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div style={{
          flex: 1,
          overflowY: 'auto',
          padding: 24,
          display: 'flex',
          flexDirection: 'column',
          gap: 28,
        }}>
          {/* GitHub */}
          <div style={sectionStyle(0)}>
            <div style={sectionLabelStyle}>GitHub</div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <GitHubIcon size={20} style={{ color: 'var(--text-secondary)' }} />
                <div>
                  <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>GitHub</div>
                  {githubStatus?.connected && githubStatus.providerUsername && (
                    <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>
                      @{githubStatus.providerUsername}
                    </div>
                  )}
                </div>
              </div>

              {githubStatus?.connected ? (
                <button
                  onClick={() => void disconnect.mutateAsync(undefined)}
                  disabled={disconnect.isPending}
                  style={{
                    background: 'rgba(239,68,68,0.1)',
                    border: '1px solid rgba(239,68,68,0.3)',
                    borderRadius: 8,
                    padding: '6px 14px',
                    fontSize: 12,
                    fontWeight: 500,
                    color: '#f87171',
                    cursor: disconnect.isPending ? 'not-allowed' : 'pointer',
                    opacity: disconnect.isPending ? 0.6 : 1,
                    transition: 'opacity 0.15s ease',
                  }}
                >
                  {disconnect.isPending ? 'Disconnecting...' : 'Disconnect'}
                </button>
              ) : (
                <a
                  href="/api/auth/github"
                  style={{
                    background: 'rgba(99,102,241,0.15)',
                    border: '1px solid rgba(99,102,241,0.35)',
                    borderRadius: 8,
                    padding: '6px 14px',
                    fontSize: 12,
                    fontWeight: 500,
                    color: 'var(--text-accent)',
                    textDecoration: 'none',
                    transition: 'opacity 0.15s ease',
                  }}
                >
                  Connect
                </a>
              )}
            </div>

            {/* Repo picker */}
            {githubStatus?.connected && (
              <div style={{ marginTop: 12 }}>
                <button
                  onClick={() => setShowRepoPicker((v) => !v)}
                  style={{
                    background: 'none',
                    border: 'none',
                    padding: 0,
                    fontSize: 12,
                    color: 'var(--text-accent)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 4,
                  }}
                >
                  {showRepoPicker ? '▲' : '▼'} Manage watched repos
                </button>

                {showRepoPicker && (
                  <div style={{ marginTop: 10 }}>
                    {reposLoading ? (
                      <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Loading repos...</div>
                    ) : !githubRepos || githubRepos.length === 0 ? (
                      <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>No repos found.</div>
                    ) : (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, maxHeight: 240, overflowY: 'auto' }}>
                        {githubRepos.map((repo) => {
                          const isSelected = selectedRepos.includes(repo.fullName);
                          return (
                            <label
                              key={repo.fullName}
                              style={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 8,
                                fontSize: 12,
                                color: 'var(--text-primary)',
                                cursor: 'pointer',
                                padding: '4px 0',
                              }}
                            >
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={() => {
                                  setSelectedRepos((prev) =>
                                    isSelected ? prev.filter((r) => r !== repo.fullName) : [...prev, repo.fullName]
                                  );
                                }}
                                style={{ accentColor: '#6366f1' }}
                              />
                              {repo.fullName}
                              {repo.private && (
                                <span style={{ fontSize: 10, color: 'var(--text-muted)', marginLeft: 4 }}>private</span>
                              )}
                            </label>
                          );
                        })}
                      </div>
                    )}

                    {githubRepos && githubRepos.length > 0 && (
                      <div style={{ marginTop: 10, display: 'flex', alignItems: 'center', gap: 10 }}>
                        <button
                          onClick={() => {
                            void updateWatchedRepos.mutateAsync(selectedRepos).then(() => {
                              setReposSaved(true);
                              reposSavedTimerRef.current = setTimeout(() => setReposSaved(false), 2000);
                            });
                          }}
                          disabled={updateWatchedRepos.isPending}
                          style={{
                            background: '#6366f1',
                            border: 'none',
                            borderRadius: 8,
                            padding: '7px 18px',
                            fontSize: 12,
                            fontWeight: 600,
                            color: '#fff',
                            cursor: updateWatchedRepos.isPending ? 'not-allowed' : 'pointer',
                            opacity: updateWatchedRepos.isPending ? 0.7 : 1,
                            transition: 'opacity 0.15s ease',
                          }}
                        >
                          {updateWatchedRepos.isPending ? 'Saving...' : 'Save'}
                        </button>
                        {reposSaved && <span style={{ fontSize: 12, color: '#34d399' }}>Saved</span>}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Divider */}
          <div style={{ ...sectionStyle(1), height: 1, background: 'rgba(255,255,255,0.08)' }} />

          {/* Slack (coming soon) */}
          <div style={{ ...sectionStyle(2), opacity: isOpen ? 0.5 : 0 }}>
            <div style={{
              fontSize: 12,
              fontWeight: 600,
              color: 'var(--text-secondary)',
              letterSpacing: '0.08em',
              textTransform: 'uppercase',
              marginBottom: 10,
            }}>
              Slack
            </div>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path d="M14.5 10a1.5 1.5 0 0 1-1.5-1.5v-5a1.5 1.5 0 0 1 3 0v5a1.5 1.5 0 0 1-1.5 1.5z" fill="currentColor" opacity=".6"/>
                  <path d="M20.5 10H19V8.5a1.5 1.5 0 0 1 3 0 1.5 1.5 0 0 1-1.5 1.5z" fill="currentColor" opacity=".6"/>
                  <path d="M9.5 14a1.5 1.5 0 0 1 1.5 1.5v5a1.5 1.5 0 0 1-3 0v-5A1.5 1.5 0 0 1 9.5 14z" fill="currentColor" opacity=".6"/>
                  <path d="M3.5 14H5v1.5a1.5 1.5 0 0 1-3 0A1.5 1.5 0 0 1 3.5 14z" fill="currentColor" opacity=".6"/>
                  <path d="M14 14.5a1.5 1.5 0 0 1 1.5-1.5h5a1.5 1.5 0 0 1 0 3h-5a1.5 1.5 0 0 1-1.5-1.5z" fill="currentColor" opacity=".6"/>
                  <path d="M14 20.5V19h1.5a1.5 1.5 0 0 1 0 3 1.5 1.5 0 0 1-1.5-1.5z" fill="currentColor" opacity=".6"/>
                  <path d="M10 9.5A1.5 1.5 0 0 1 8.5 11h-5a1.5 1.5 0 0 1 0-3h5A1.5 1.5 0 0 1 10 9.5z" fill="currentColor" opacity=".6"/>
                  <path d="M10 3.5V5H8.5a1.5 1.5 0 0 1 0-3A1.5 1.5 0 0 1 10 3.5z" fill="currentColor" opacity=".6"/>
                </svg>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 500, color: 'var(--text-primary)' }}>Slack</div>
                  <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 2 }}>Coming soon</div>
                </div>
              </div>
              <span style={{
                fontSize: 11,
                fontWeight: 500,
                color: 'var(--text-muted)',
                background: 'rgba(255,255,255,0.06)',
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 6,
                padding: '4px 10px',
              }}>
                Coming soon
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
