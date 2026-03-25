import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchBillsDueYear, markBillPaid, unmarkBillPaid } from '../api/client';
import type { MarkPaidPayload } from '../api/client';
import type { BillDue } from '../types/dashboard';
import type { BillsYearResponse } from '../api/client';

export function useBillsDueYear(year: number): {
  monthBills: Record<number, BillDue[]>;
  isLoading: boolean;
  markPaid: (billId: string, payload: MarkPaidPayload) => void;
  unmark: (paymentId: string, billId: string, computedDueDate: string) => void;
  isPending: boolean;
} {
  const queryClient = useQueryClient();
  const queryKey = ['billsDueYear', year];

  const { data, isLoading } = useQuery({
    queryKey,
    queryFn: () => fetchBillsDueYear(year),
    staleTime: 5 * 60_000,
  });

  const monthBills: Record<number, BillDue[]> = {};
  if (data) {
    for (const entry of data.months) {
      monthBills[entry.month] = entry.bills;
    }
  }

  const markPaidMutation = useMutation({
    mutationFn: ({ billId, payload }: { billId: string; payload: MarkPaidPayload }) =>
      markBillPaid(billId, payload),
    onMutate: async ({ billId, payload }) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<BillsYearResponse>(queryKey);
      queryClient.setQueryData<BillsYearResponse>(queryKey, (old) => {
        if (!old) return old;
        return {
          ...old,
          months: old.months.map((entry) => ({
            ...entry,
            bills: entry.bills.map((b) =>
              b.id === billId && b.computedDueDate === payload.computedDueDate
                ? { ...b, isPaid: true, paidDate: payload.paidDate, paidNote: payload.note ?? null }
                : b
            ),
          })),
        };
      });
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const unmarkMutation = useMutation({
    mutationFn: ({ paymentId }: { paymentId: string; billId: string; computedDueDate: string }) =>
      unmarkBillPaid(paymentId),
    onMutate: async ({ billId, computedDueDate }) => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<BillsYearResponse>(queryKey);
      queryClient.setQueryData<BillsYearResponse>(queryKey, (old) => {
        if (!old) return old;
        return {
          ...old,
          months: old.months.map((entry) => ({
            ...entry,
            bills: entry.bills.map((b) =>
              b.id === billId && b.computedDueDate === computedDueDate
                ? { ...b, isPaid: false, paidDate: null, paidNote: null, paymentId: null }
                : b
            ),
          })),
        };
      });
      return { previous };
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(queryKey, context.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  return {
    monthBills,
    isLoading,
    markPaid: (billId, payload) => markPaidMutation.mutate({ billId, payload }),
    unmark: (paymentId, billId, computedDueDate) =>
      unmarkMutation.mutate({ paymentId, billId, computedDueDate }),
    isPending: markPaidMutation.isPending || unmarkMutation.isPending,
  };
}
