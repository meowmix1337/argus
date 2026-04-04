import { vi, describe, it, expect, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { type ReactNode } from 'react'
import { useTasks } from './useTasks'
import { toggleTask, createTask, deleteTask } from '../api/client'
import type { Task } from '../types/dashboard'

vi.mock('../api/client')

function makeWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
}

const sampleTask: Task = { id: '1', text: 'Write tests', done: false, priority: 'medium' }

describe('useTasks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('toggle', () => {
    it('optimistically sets done=true before server responds', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      // Never resolves: waitFor can only pass once onMutate applied the optimistic update
      vi.mocked(toggleTask).mockImplementation(() => new Promise<Task>(() => {}))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.toggle.mutate({ id: '1', done: true }) })

      await waitFor(() => {
        expect(qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks[0].done).toBe(true)
      })
    })

    it('rolls back to original state on server error', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      vi.mocked(toggleTask).mockRejectedValue(new Error('server error'))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.toggle.mutate({ id: '1', done: true }) })

      await waitFor(() => {
        expect(qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks[0].done).toBe(false)
      })
    })
  })

  describe('create', () => {
    it('optimistically adds a temp task before server responds', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      vi.mocked(createTask).mockImplementation(() => new Promise<Task>(() => {}))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.create.mutate({ text: 'New task', priority: 'low' }) })

      await waitFor(() => {
        const tasks = qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks
        expect(tasks).toHaveLength(2)
        expect(tasks?.[1].text).toBe('New task')
      })
    })

    it('rolls back to original task list on server error', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      vi.mocked(createTask).mockRejectedValue(new Error('server error'))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.create.mutate({ text: 'New task', priority: 'low' }) })

      await waitFor(() => {
        expect(qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks).toHaveLength(1)
      })
    })
  })

  describe('remove', () => {
    it('optimistically removes the task before server responds', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      vi.mocked(deleteTask).mockImplementation(() => new Promise<void>(() => {}))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.remove.mutate('1') })

      await waitFor(() => {
        expect(qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks).toHaveLength(0)
      })
    })

    it('rolls back to original task list on server error', async () => {
      const qc = makeQueryClient()
      qc.setQueryData(['dashboard'], { tasks: [sampleTask] })
      vi.mocked(deleteTask).mockRejectedValue(new Error('server error'))

      const { result } = renderHook(() => useTasks(), { wrapper: makeWrapper(qc) })

      act(() => { result.current.remove.mutate('1') })

      await waitFor(() => {
        expect(qc.getQueryData<{ tasks: Task[] }>(['dashboard'])?.tasks).toHaveLength(1)
      })
    })
  })
})
