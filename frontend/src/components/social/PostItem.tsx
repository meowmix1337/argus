import React from 'react';
import type { Post } from '../../types/dashboard';
import { usePosts } from '../../hooks/usePosts';
import { useAuth } from '../../hooks/useAuth';

interface PostItemProps {
  post: Post;
}

function timeAgo(createdAt: string): string {
  const diff = Math.floor((Date.now() - new Date(createdAt).getTime()) / 1000);
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
}

export function PostItem({ post }: PostItemProps): React.ReactElement {
  const { like, remove } = usePosts();
  const { user } = useAuth();
  const isOwner = user?.user_id === post.userId;

  return (
    <div style={{
      padding: '14px 0',
      borderBottom: '1px solid var(--border-subtle)',
      display: 'flex',
      gap: 12,
    }}>
      {/* Avatar */}
      <div style={{
        width: 36,
        height: 36,
        borderRadius: '50%',
        background: 'rgba(99,102,241,0.18)',
        border: '1px solid rgba(99,102,241,0.25)',
        flexShrink: 0,
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: 14,
        color: 'var(--text-accent)',
        fontWeight: 600,
      }}>
        {post.userAvatar ? (
          <img
            src={post.userAvatar}
            alt={post.userName}
            referrerPolicy="no-referrer"
            style={{ width: '100%', height: '100%', objectFit: 'cover' }}
          />
        ) : (
          post.userName.charAt(0).toUpperCase()
        )}
      </div>

      {/* Content */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, marginBottom: 4 }}>
          <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
            {post.userName}
          </span>
          <span style={{
            fontSize: 11,
            color: 'var(--text-muted)',
            fontFamily: "'JetBrains Mono', monospace",
          }}>
            {timeAgo(post.createdAt)}
          </span>
        </div>
        <p style={{
          margin: 0,
          fontSize: 13,
          color: 'var(--text-primary)',
          lineHeight: 1.5,
          wordBreak: 'break-word',
        }}>
          {post.content}
        </p>

        {/* Actions */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginTop: 8 }}>
          <button
            onClick={() => like.mutate(post.id)}
            disabled={like.isPending}
            aria-label={post.likedByMe ? 'Unlike post' : 'Like post'}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 4,
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              padding: 0,
              fontSize: 12,
              color: post.likedByMe ? '#ec4899' : 'var(--text-muted)',
              transition: 'color 0.15s',
            }}
          >
            <span style={{ fontSize: 14 }}>{post.likedByMe ? '♥' : '♡'}</span>
            {post.likeCount > 0 && (
              <span style={{ fontFamily: "'JetBrains Mono', monospace" }}>{post.likeCount}</span>
            )}
          </button>

          {isOwner && (
            <button
              onClick={() => remove.mutate(post.id)}
              disabled={remove.isPending}
              style={{
                background: 'none',
                border: 'none',
                cursor: 'pointer',
                padding: 0,
                fontSize: 12,
                color: 'var(--text-muted)',
                opacity: 0.6,
                transition: 'opacity 0.15s',
              }}
              title="Delete post"
              aria-label="Delete post"
            >
              ×
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
