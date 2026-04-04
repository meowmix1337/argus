import { vi, describe, it, expect, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { type ReactNode } from 'react'
import { useSymbolSearch } from './useSymbolSearch'
import { searchSymbols } from '../api/client'

vi.mock('../api/client')

function makeWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

describe('useSymbolSearch', () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
  })

  it('does not call searchSymbols before the 350ms debounce settles', async () => {
    vi.useFakeTimers()
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    vi.mocked(searchSymbols).mockResolvedValue({ results: [] })

    const { result } = renderHook(() => useSymbolSearch(), { wrapper: makeWrapper(qc) })

    await act(async () => {
      result.current.setSearchQuery('TS')
    })

    expect(vi.mocked(searchSymbols)).not.toHaveBeenCalled()
  })

  it('calls searchSymbols after the 350ms debounce settles', async () => {
    vi.useFakeTimers()
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    vi.mocked(searchSymbols).mockResolvedValue({ results: [] })

    const { result } = renderHook(() => useSymbolSearch(), { wrapper: makeWrapper(qc) })

    await act(async () => {
      result.current.setSearchQuery('TS')
    })

    await act(async () => {
      vi.advanceTimersByTime(350)
    })

    vi.useRealTimers()

    await waitFor(() => {
      expect(vi.mocked(searchSymbols)).toHaveBeenCalledWith('TS')
    })
  })
})
