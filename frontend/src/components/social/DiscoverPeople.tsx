import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { fetchFollowers } from '../../api/client';
import { FollowButton } from './FollowButton';
import { useAuth } from '../../hooks/useAuth';

// Shows up to 5 recently-joined followers of the current user's contacts as
// a "people to follow" suggestion. Since we don't have a dedicated discover
// endpoint yet, we show the viewer's own followers (people who follow you
// but you may not follow back) as discover suggestions.
export function DiscoverPeople(): React.ReactElement | null {
  const { user } = useAuth();

  const { data, isLoading } = useQuery({
    queryKey: ['discoverPeople', user?.user_id],
    queryFn: () => fetchFollowers(user!.user_id, 10, 0),
    enabled: !!user?.user_id,
    staleTime: 120_000,
  });

  if (isLoading || !data?.users.length) return null;

  return (
    <div style={{
      background: 'var(--bg-card)',
      border: '1px solid var(--bg-card-border)',
      borderRadius: 12,
      padding: 16,
      backdropFilter: 'blur(20px)',
    }}>
      <div style={{
        fontSize: 11,
        fontWeight: 600,
        color: 'var(--text-secondary)',
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
        marginBottom: 12,
      }}>
        People you may know
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {data.users.slice(0, 5).map((person) => (
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
                  style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                />
              ) : (
                person.name.charAt(0).toUpperCase()
              )}
            </div>
            <span style={{ flex: 1, fontSize: 13, color: 'var(--text-primary)', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {person.name}
            </span>
            <FollowButton targetId={person.id} />
          </div>
        ))}
      </div>
    </div>
  );
}
