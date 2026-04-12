import React, { useState, useRef, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { searchPosts } from '../../api/client';
import { PostComposer } from './PostComposer';
import { FeedList } from './FeedList';
import { PostItem } from './PostItem';
import { DiscoverPeople } from './DiscoverPeople';
import type { Post } from '../../types/dashboard';

const SEARCH_DEBOUNCE_MS = 400;

export function SocialSection(): React.ReactElement {
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);

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

  const { data: searchData, isLoading: searchLoading, isFetching: searchFetching } = useQuery({
    queryKey: ['postSearch', debouncedQuery],
    queryFn: () => searchPosts(debouncedQuery),
    enabled: debouncedQuery.length > 0,
    staleTime: 30_000,
  });

  const hasQuery = debouncedQuery.length > 0;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* Section header */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        paddingBottom: 12,
        borderBottom: '1px solid var(--header-border)',
      }}>
        <span style={{ fontSize: 14, color: 'var(--text-accent)', opacity: 0.8 }}>◈</span>
        <span style={{
          fontSize: 13,
          fontWeight: 600,
          color: 'var(--text-secondary)',
          letterSpacing: '0.04em',
          textTransform: 'uppercase',
        }}>
          Social
        </span>
      </div>

      {/* Search bar — always visible */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        background: 'var(--bg-card)',
        border: '1px solid var(--bg-card-border)',
        borderRadius: 10,
        padding: '8px 12px',
      }}>
        <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>⌕</span>
        <input
          type="text"
          value={query}
          onChange={handleChange}
          placeholder="Search posts…"
          aria-label="Search posts"
          style={{
            flex: 1,
            background: 'transparent',
            border: 'none',
            outline: 'none',
            fontSize: 13,
            color: 'var(--text-primary)',
            caretColor: 'var(--text-accent)',
            fontFamily: "'DM Sans', 'Helvetica Neue', sans-serif",
          }}
        />
        {(searchLoading || searchFetching) && debouncedQuery && (
          <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>…</span>
        )}
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

      {/* Content: search results when typing, otherwise composer + feed */}
      {hasQuery ? (
        <div>
          {!searchLoading && !searchFetching && searchData?.posts.length === 0 && (
            <div style={{ padding: '12px 0', textAlign: 'center', fontSize: 12, color: 'var(--text-muted)' }}>
              No results for "{debouncedQuery}"
            </div>
          )}
          {searchData?.posts.map((post: Post) => (
            <PostItem key={post.id} post={post} />
          ))}
        </div>
      ) : (
        <>
          <PostComposer />
          <div style={{
            background: 'var(--bg-card)',
            border: '1px solid var(--bg-card-border)',
            borderRadius: 12,
            padding: '0 16px',
          }}>
            <FeedList />
          </div>
          <DiscoverPeople />
        </>
      )}
    </div>
  );
}
