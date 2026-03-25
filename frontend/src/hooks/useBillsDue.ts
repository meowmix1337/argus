import { useQuery, keepPreviousData } from '@tanstack/react-query';
import { fetchBillsDue } from '../api/client';
import type { BillDue } from '../types/dashboard';

export function useBillsDue(year: number, month: number): { bills: BillDue[]; isLoading: boolean; isFetching: boolean } {
  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['billsDue', year, month],
    queryFn: () => fetchBillsDue(year, month),
    staleTime: 60_000,
    placeholderData: keepPreviousData,
  });

  return { bills: data?.bills ?? [], isLoading, isFetching };
}
