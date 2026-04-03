import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  getGitHubIntegrationStatus,
  disconnectGitHub as apiDisconnect,
  fetchGitHubRepos,
  updateWatchedRepos as apiUpdateWatchedRepos,
} from '../api/client';

export function useGitHubIntegration() {
  return useQuery({
    queryKey: ['integrations', 'github'],
    queryFn: getGitHubIntegrationStatus,
    staleTime: 60_000,
  });
}

export function useGitHubRepos(enabled: boolean) {
  return useQuery({
    queryKey: ['integrations', 'github', 'repos'],
    queryFn: fetchGitHubRepos,
    enabled,
    staleTime: 60_000,
  });
}

export function useGitHubMutations() {
  const queryClient = useQueryClient();

  function invalidateIntegration() {
    queryClient.invalidateQueries({ queryKey: ['integrations', 'github'] });
  }

  const disconnect = useMutation({
    mutationFn: apiDisconnect,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['integrations'] });
    },
  });

  const updateWatchedRepos = useMutation({
    mutationFn: (repos: string[]) => apiUpdateWatchedRepos(repos),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['integrations', 'github', 'repos'] });
      invalidateIntegration();
    },
  });

  return { disconnect, updateWatchedRepos };
}
