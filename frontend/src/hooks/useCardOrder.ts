import { useState } from 'react';

export type CardId = 'weather' | 'calendar' | 'tasks' | 'news' | 'quote' | 'bills';

const DEFAULT_ORDER: CardId[] = ['weather', 'calendar', 'tasks', 'news', 'quote', 'bills'];
const STORAGE_KEY = 'dashboard-card-order';

function loadOrder(): CardId[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_ORDER;
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return DEFAULT_ORDER;
    // Validate all entries are known card IDs, deduplicate, and append any new cards
    const seen = new Set<CardId>();
    const valid = parsed.filter((id): id is CardId => {
      if (!DEFAULT_ORDER.includes(id as CardId)) return false;
      if (seen.has(id as CardId)) return false;
      seen.add(id as CardId);
      return true;
    });
    // Append any new cards not in persisted order (future-proofing)
    const missing = DEFAULT_ORDER.filter(id => !valid.includes(id));
    return [...valid, ...missing];
  } catch {
    return DEFAULT_ORDER;
  }
}

export function useCardOrder(): [CardId[], (order: CardId[]) => void] {
  const [order, setOrderState] = useState<CardId[]>(loadOrder);

  function setCardOrder(newOrder: CardId[]): void {
    setOrderState(newOrder);
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(newOrder));
    } catch {
      // localStorage unavailable — order still works in-memory
    }
  }

  return [order, setCardOrder];
}
