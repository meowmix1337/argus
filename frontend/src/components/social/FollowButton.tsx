import React, { useState } from 'react';
import { useFollowStatus, useFollowMutations } from '../../hooks/useFollow';
import { useAuth } from '../../hooks/useAuth';

interface FollowButtonProps {
  targetId: string;
}

export function FollowButton({ targetId }: FollowButtonProps): React.ReactElement | null {
  const { user } = useAuth();
  const { data, isLoading } = useFollowStatus(targetId);
  const { follow, unfollow } = useFollowMutations();
  const [hovered, setHovered] = useState(false);

  // Don't show follow button for yourself
  if (!user || user.user_id === targetId) return null;
  if (isLoading) return null;

  const following = data?.following ?? false;
  const isPending = follow.isPending || unfollow.isPending;

  function handleClick() {
    if (isPending) return;
    if (following) {
      unfollow.mutate(targetId);
    } else {
      follow.mutate(targetId);
    }
  }

  const label = following ? (hovered ? 'Unfollow' : 'Following') : 'Follow';

  return (
    <button
      onClick={handleClick}
      disabled={isPending}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        background: following
          ? (hovered ? 'rgba(239,68,68,0.1)' : 'rgba(99,102,241,0.1)')
          : 'rgba(99,102,241,0.18)',
        border: `1px solid ${following ? (hovered ? 'rgba(239,68,68,0.4)' : 'rgba(99,102,241,0.3)') : 'rgba(99,102,241,0.4)'}`,
        borderRadius: 20,
        padding: '4px 12px',
        fontSize: 11,
        fontWeight: 600,
        color: following ? (hovered ? '#ef4444' : 'var(--text-accent)') : 'var(--text-accent)',
        cursor: isPending ? 'not-allowed' : 'pointer',
        transition: 'all 0.15s',
        whiteSpace: 'nowrap',
        flexShrink: 0,
      }}
    >
      {label}
    </button>
  );
}
