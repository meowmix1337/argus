import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createBill as apiCreateBill,
  deleteBill as apiDeleteBill,
  fetchBills,
  updateBill as apiUpdateBill,
  type BillPayload,
} from '../api/client';
import type { Bill } from '../types/dashboard';

export function useBills() {
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['bills'],
    queryFn: () => fetchBills(),
    staleTime: 30_000,
  });

  const create = useMutation({
    mutationFn: (payload: BillPayload) => apiCreateBill(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['bills'] });
      queryClient.invalidateQueries({ queryKey: ['billsDueYear'] });
    },
  });

  const update = useMutation({
    mutationFn: ({ id, ...payload }: { id: string } & BillPayload) =>
      apiUpdateBill(id, payload),
    onSuccess: (updated) => {
      queryClient.setQueryData<{ bills: Bill[] }>(['bills'], (old) => {
        if (!old) return old;
        return { ...old, bills: old.bills.map((b) => (b.id === updated.id ? updated : b)) };
      });
      queryClient.invalidateQueries({ queryKey: ['billsDueYear'] });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => apiDeleteBill(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['bills'] });
      const previous = queryClient.getQueryData<{ bills: Bill[] }>(['bills']);
      queryClient.setQueryData<{ bills: Bill[] }>(['bills'], (old) => {
        if (!old) return old;
        return { ...old, bills: old.bills.filter((b) => b.id !== id) };
      });
      return { previous };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.previous) queryClient.setQueryData(['bills'], ctx.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['bills'] });
      queryClient.invalidateQueries({ queryKey: ['billsDueYear'] });
    },
  });

  return { bills: data?.bills ?? [], total: data?.total ?? 0, isLoading, create, update, remove };
}
