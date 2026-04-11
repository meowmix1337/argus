import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { followUser, unfollowUser, getFollowStatus, fetchFollowing } from '../api/client';

export function useFollowStatus(targetId: string) {
  return useQuery({
    queryKey: ['followStatus', targetId],
    queryFn: () => getFollowStatus(targetId),
    staleTime: 60_000,
  });
}

export function useFollowing(userId: string) {
  return useQuery({
    queryKey: ['following', userId],
    queryFn: () => fetchFollowing(userId, 50, 0),
    staleTime: 60_000,
  });
}

export function useFollowMutations() {
  const queryClient = useQueryClient();

  const follow = useMutation({
    mutationFn: (targetId: string) => followUser(targetId),
    onSuccess: (_data, targetId) => {
      queryClient.setQueryData(['followStatus', targetId], { following: true });
      queryClient.invalidateQueries({ queryKey: ['following'] });
    },
  });

  const unfollow = useMutation({
    mutationFn: (targetId: string) => unfollowUser(targetId),
    onSuccess: (_data, targetId) => {
      queryClient.setQueryData(['followStatus', targetId], { following: false });
      queryClient.invalidateQueries({ queryKey: ['following'] });
    },
  });

  return { follow, unfollow };
}
