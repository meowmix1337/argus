import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { searchPosts } from '../../api/client';
import { PostItem } from './PostItem';
import type { Post } from '../../types/dashboard';

export function SearchBar(): React.ReactElement {
  const [query, setQuery] = useState('');
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const debounceRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    const val = e.target.value;
    setQuery(val);
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => setDebouncedQuery(val.trim()), 400);
  }

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['postSearch', debouncedQuery],
    queryFn: () => searchPosts(debouncedQuery),
    enabled: debouncedQuery.length > 0,
    staleTime: 30_000,
  });

  const showResults = debouncedQuery.length > 0;

  return (
    <div>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        background: 'var(--bg-card)',
        border: '1px solid var(--bg-card-border)',
        borderRadius: 10,
        padding: '8px 12px',
        backdropFilter: 'blur(20px)',
      }}>
        <span style={{ fontSize: 13, color: 'var(--text-muted)' }}>⌕</span>
        <input
          type="text"
          value={query}
          onChange={handleChange}
          placeholder="Search posts…"
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
        {(isLoading || isFetching) && debouncedQuery && (
          <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>…</span>
        )}
        {query && (
          <button
            onClick={() => { setQuery(''); setDebouncedQuery(''); }}
            style={{
              background: 'none', border: 'none', cursor: 'pointer',
              fontSize: 14, color: 'var(--text-muted)', padding: 0, lineHeight: 1,
            }}
          >
            ×
          </button>
        )}
      </div>

      {showResults && (
        <div style={{ marginTop: 8 }}>
          {!isLoading && !isFetching && data?.posts.length === 0 && (
            <div style={{ padding: '12px 0', textAlign: 'center', fontSize: 12, color: 'var(--text-muted)' }}>
              No results for "{debouncedQuery}"
            </div>
          )}
          {data?.posts.map((post: Post) => (
            <PostItem key={post.id} post={post} />
          ))}
        </div>
      )}
    </div>
  );
}
