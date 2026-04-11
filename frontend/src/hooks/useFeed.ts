import { useInfiniteQuery } from '@tanstack/react-query';
import { fetchFeed } from '../api/client';
import type { FeedResponse } from '../types/dashboard';

const FEED_LIMIT = 20;

export function useFeed() {
  return useInfiniteQuery({
    queryKey: ['feed'] as const,
    queryFn: ({ pageParam }: { pageParam: string | undefined }) =>
      fetchFeed(pageParam, FEED_LIMIT),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage: FeedResponse) => lastPage.nextCursor ?? undefined,
    staleTime: 30_000,
    refetchInterval: 30_000,
  });
}
