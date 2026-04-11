import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { InfiniteData } from '@tanstack/react-query';
import { createPost, deletePost, toggleLike } from '../api/client';
import type { FeedResponse } from '../types/dashboard';

export function usePosts() {
  const queryClient = useQueryClient();

  const create = useMutation({
    mutationFn: ({ content, parentPostId }: { content: string; parentPostId?: string | null }) =>
      createPost(content, parentPostId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feed'] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => deletePost(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['feed'] });
    },
  });

  const like = useMutation({
    mutationFn: (postId: string) => toggleLike(postId),
    onSuccess: (updatedPost) => {
      // Patch cached feed pages so the like state updates instantly.
      queryClient.setQueriesData<InfiniteData<FeedResponse>>(
        { queryKey: ['feed'] },
        (old) => {
          if (!old) return old;
          return {
            ...old,
            pages: old.pages.map((page) => ({
              ...page,
              posts: page.posts.map((p) =>
                p.id === updatedPost.id ? updatedPost : p
              ),
            })),
          };
        }
      );
    },
  });

  return { create, remove, like };
}
