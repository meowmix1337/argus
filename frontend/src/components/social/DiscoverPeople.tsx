import React, { useState, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchFollowers, searchUsers } from '../../api/client';
import { FollowButton } from './FollowButton';
import { useAuth } from '../../hooks/useAuth';
import type { UserSummary } from '../../types/dashboard';

const SEARCH_DEBOUNCE_MS = 300;

export function DiscoverPeople(): React.ReactElement {
  const { user } = useAuth();
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const val = e.target.value;
    setQuery(val);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setDebouncedQuery(val.trim()), SEARCH_DEBOUNCE_MS);
  }

  function clearSearch() {
    setQuery('');
    setDebouncedQuery('');
    if (debounceRef.current) clearTimeout(debounceRef.current);
  }

  const isSearching = debouncedQuery.length >= 2;

  const { data: searchData, isLoading: searchLoading } = useQuery({
    queryKey: ['userSearch', debouncedQuery],
    queryFn: () => searchUsers(debouncedQuery, 20),
    enabled: isSearching,
    staleTime: 30_000,
  });

  const { data: followersData } = useQuery({
    queryKey: ['discoverPeople', user?.user_id],
    queryFn: () => fetchFollowers(user!.user_id, 10, 0),
    enabled: !!user?.user_id && !isSearching,
    staleTime: 120_000,
  });

  const results: UserSummary[] = isSearching
    ? (searchData?.users ?? [])
    : (followersData?.users ?? []);

  const label = isSearching ? 'Results' : 'People you may know';
  const showEmpty = isSearching && !searchLoading && results.length === 0;
  const showDefault = !isSearching && results.length === 0;

  return (
    <div style={{
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 12,
      padding: 16,
    }}>
      <div style={{
        fontSize: 11,
        fontWeight: 600,
        color: 'var(--text-secondary)',
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
        marginBottom: 10,
      }}>
        Discover People
      </div>

      {/* Search input */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        background: 'rgba(255,255,255,0.04)',
        border: '1px solid var(--bg-card-border)',
        borderRadius: 8,
        padding: '6px 10px',
        marginBottom: 12,
      }}>
        <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>⌕</span>
        <input
          type="text"
          value={query}
          onChange={handleChange}
          placeholder="Find people by name…"
          aria-label="Search for people"
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            fontSize: 12,
            color: 'var(--text-primary)',
            caretColor: 'var(--text-accent)',
            fontFamily: "'DM Sans', 'Helvetica Neue', sans-serif",
          }}
        />
        {query && (
          <button
            onClick={clearSearch}
            aria-label="Clear search"
            style={{
              background: 'none', border: 'none', cursor: 'pointer',
              fontSize: 14, color: 'var(--text-muted)', padding: 0, lineHeight: 1,
            }}
          >
            ×
          </button>
        )}
      </div>

      {/* Results */}
      {showEmpty && (
        <div style={{ fontSize: 12, color: 'var(--text-muted)', textAlign: 'center', padding: '8px 0' }}>
          No results for "{debouncedQuery}"
        </div>
      )}

      {showDefault && (
        <div style={{ fontSize: 12, color: 'var(--text-muted)', textAlign: 'center', padding: '8px 0' }}>
          Search to find people to follow
        </div>
      )}

      {results.length > 0 && (
        <>
          <div style={{
            fontSize: 10,
            fontWeight: 600,
            color: 'var(--text-muted)',
            letterSpacing: '0.05em',
            textTransform: 'uppercase',
            marginBottom: 8,
          }}>
            {label}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
            {results.map((person) => (
              <div key={person.id} style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <div style={{
                  width: 32,
                  height: 32,
                  borderRadius: '50%',
                  background: 'rgba(99,102,241,0.18)',
                  border: '1px solid rgba(99,102,241,0.25)',
                  flexShrink: 0,
                  overflow: 'hidden',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 12,
                  color: 'var(--text-accent)',
                  fontWeight: 600,
                }}>
                  {person.avatarUrl ? (
                    <img
                      src={person.avatarUrl}
                      alt={person.name}
                      referrerPolicy="no-referrer"
                      style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                    />
                  ) : (
                    person.name.charAt(0).toUpperCase()
                  )}
                </div>
                <span style={{
                  flex: 1, fontSize: 13, color: 'var(--text-primary)',
                  minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                }}>
                  {person.name}
                </span>
                <FollowButton targetId={person.id} />
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
