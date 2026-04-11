import React, { useState } from 'react';
import { usePosts } from '../../hooks/usePosts';

const MAX_LENGTH = 128;

export function PostComposer(): React.ReactElement {
  const [content, setContent] = useState('');
  const { create } = usePosts();

  const remaining = MAX_LENGTH - content.length;
  const isOverLimit = remaining < 0;
  const isEmpty = content.trim().length === 0;

  function handleSubmit() {
    if (isEmpty || isOverLimit || create.isPending) return;
    create.mutate({ content: content.trim() });
    setContent('');
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      handleSubmit();
    }
  }

  return (
    <div style={{
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 12,
      padding: 16,
      backdropFilter: 'blur(20px)',
    }}>
      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="What's on your mind?"
        rows={2}
        style={{
          width: '100%',
          background: 'transparent',
          border: 'none',
          outline: 'none',
          resize: 'none',
          fontSize: 14,
          color: 'var(--text-primary)',
          caretColor: 'var(--text-accent)',
          fontFamily: "'DM Sans', 'Helvetica Neue', sans-serif",
          lineHeight: 1.5,
          boxSizing: 'border-box',
        }}
      />
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        gap: 12,
        marginTop: 8,
        paddingTop: 8,
        borderTop: '1px solid var(--border-subtle)',
      }}>
        <span style={{
          fontSize: 11,
          fontFamily: "'JetBrains Mono', monospace",
          color: isOverLimit ? '#ef4444' : remaining <= 20 ? '#f59e0b' : 'var(--text-muted)',
        }}>
          {remaining}
        </span>
        <button
          onClick={handleSubmit}
          disabled={isEmpty || isOverLimit || create.isPending}
          style={{
            background: isEmpty || isOverLimit ? 'rgba(99,102,241,0.08)' : 'rgba(99,102,241,0.18)',
            border: '1px solid rgba(99,102,241,0.3)',
            borderRadius: 7,
            padding: '5px 14px',
            fontSize: 12,
            fontWeight: 600,
            color: isEmpty || isOverLimit ? 'var(--text-muted)' : 'var(--text-accent)',
            cursor: isEmpty || isOverLimit || create.isPending ? 'not-allowed' : 'pointer',
            transition: 'background 0.15s, color 0.15s',
          }}
        >
          {create.isPending ? 'Posting…' : 'Post'}
        </button>
      </div>
    </div>
  );
}
