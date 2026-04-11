import React, { useEffect, useRef } from 'react';
import { useFeed } from '../../hooks/useFeed';
import { PostItem } from './PostItem';
import type { Post } from '../../types/dashboard';

export function FeedList(): React.ReactElement {
  const { data, isLoading, isError, fetchNextPage, hasNextPage, isFetchingNextPage } = useFeed();
  const sentinelRef = useRef<HTMLDivElement>(null);

  // Infinite scroll via IntersectionObserver
  useEffect(() => {
    const el = sentinelRef.current;
    if (!el) return;
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasNextPage && !isFetchingNextPage) {
          void fetchNextPage();
        }
      },
      { threshold: 0 }
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  if (isLoading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
        {Array.from({ length: 5 }).map((_, i) => (
          <FeedSkeleton key={i} />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div style={{ padding: '20px 0', textAlign: 'center', fontSize: 13, color: 'var(--text-muted)' }}>
        Failed to load feed
      </div>
    );
  }

  const posts: Post[] = data?.pages.flatMap((page) => page.posts) ?? [];

  if (posts.length === 0) {
    return (
      <div style={{ padding: '32px 0', textAlign: 'center', fontSize: 13, color: 'var(--text-muted)' }}>
        No posts yet. Follow some people or create your first post.
      </div>
    );
  }

  return (
    <div>
      {posts.map((post) => (
        <PostItem key={post.id} post={post} />
      ))}
      <div ref={sentinelRef} style={{ paddingTop: 4 }}>
        {isFetchingNextPage && (
          <div style={{ textAlign: 'center', fontSize: 12, color: 'var(--text-muted)', padding: '8px 0' }}>
            Loading more…
          </div>
        )}
        {!hasNextPage && posts.length > 0 && (
          <div style={{ textAlign: 'center', fontSize: 11, color: 'var(--text-muted)', padding: '12px 0', opacity: 0.5 }}>
            You're all caught up
          </div>
        )}
      </div>
    </div>
  );
}

function FeedSkeleton(): React.ReactElement {
  return (
    <div style={{ padding: '14px 0', borderBottom: '1px solid var(--border-subtle)', display: 'flex', gap: 12 }}>
      <div style={{
        width: 36, height: 36, borderRadius: '50%',
        background: 'var(--bg-skeleton)', flexShrink: 0,
        animation: 'pulse 1.5s ease-in-out infinite',
      }} />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div style={{ width: '30%', height: 12, borderRadius: 4, background: 'var(--bg-skeleton)', animation: 'pulse 1.5s ease-in-out infinite' }} />
        <div style={{ width: '85%', height: 12, borderRadius: 4, background: 'var(--bg-skeleton)', animation: 'pulse 1.5s ease-in-out infinite' }} />
        <div style={{ width: '60%', height: 12, borderRadius: 4, background: 'var(--bg-skeleton)', animation: 'pulse 1.5s ease-in-out infinite' }} />
      </div>
    </div>
  );
}
