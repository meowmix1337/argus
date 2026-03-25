import { useQuery } from '@tanstack/react-query';
import { fetchBillsDueYear } from '../api/client';
import type { BillDue } from '../types/dashboard';

export function useBillsDueYear(year: number): { monthBills: Record<number, BillDue[]>; isLoading: boolean } {
  const { data, isLoading } = useQuery({
    queryKey: ['billsDueYear', year],
    queryFn: () => fetchBillsDueYear(year),
    staleTime: 5 * 60_000,
  });

  const monthBills: Record<number, BillDue[]> = {};
  if (data) {
    for (const entry of data.months) {
      monthBills[entry.month] = entry.bills;
    }
  }

  return { monthBills, isLoading };
}
