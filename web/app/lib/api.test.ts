import { describe, it, expect, beforeEach, vi } from 'vitest'
import { api } from './api'

// Mock fetch globally
global.fetch = vi.fn()

describe('API Client', () => {
  const mockFetch = global.fetch as ReturnType<typeof vi.fn>

  beforeEach(() => {
    mockFetch.mockClear()
    // Mock localStorage for JWT token
    global.localStorage = {
      getItem: vi.fn(() => 'mock-token'),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      key: vi.fn(),
      length: 0,
    }
  })

  describe('Scripts API', () => {
    it('should return paginated response for getScripts', async () => {
      const mockResponse = {
        data: [
          {
            id: 1,
            name: 'test-script',
            description: 'Test',
            content: 'log.info("test");',
            enabled: true,
            provisioned_from_config: false,
            created_at: '2025-01-01T00:00:00Z',
            updated_at: '2025-01-01T00:00:00Z',
            triggers: [],
          },
        ],
        pagination: {
          total: 1,
          page: 1,
          page_size: 25,
          total_pages: 1,
        },
      }

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await api.getScripts()

      // Verify correct structure
      expect(result).toHaveProperty('data')
      expect(result).toHaveProperty('pagination')
      expect(result.data).toBeInstanceOf(Array)
      expect(result.data).toHaveLength(1)
      expect(result.pagination.total).toBe(1)
      expect(result.pagination.page).toBe(1)
    })

    it('should handle pagination params for getScripts', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], pagination: { total: 0, page: 2, page_size: 10, total_pages: 0 } }),
      })

      await api.getScripts({ page: 2, pageSize: 10 })

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/scripts?page=2&pageSize=10'),
        expect.any(Object)
      )
    })

    it('should return paginated response for getScriptLogs', async () => {
      const mockResponse = {
        data: [
          {
            id: 1,
            script_id: 1,
            trigger_type: 'on_publish',
            level: 'info',
            message: 'Test log',
            execution_time_ms: 10,
            created_at: '2025-01-01T00:00:00Z',
          },
        ],
        pagination: {
          total: 1,
          page: 1,
          page_size: 50,
          total_pages: 1,
        },
      }

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await api.getScriptLogs(1, { page: 1, page_size: 50 })

      // Verify correct structure (this would have caught the bug!)
      expect(result).toHaveProperty('data')
      expect(result).toHaveProperty('pagination')
      expect(result.data).toBeInstanceOf(Array)
      expect(result.pagination).toHaveProperty('total')
      expect(result.pagination).toHaveProperty('total_pages')

      // Verify we DON'T have the wrong structure
      expect(result).not.toHaveProperty('logs')
      expect(result).not.toHaveProperty('total')
    })

    it('should handle log level filtering', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], pagination: { total: 0, page: 1, page_size: 50, total_pages: 0 } }),
      })

      await api.getScriptLogs(1, { page: 1, page_size: 50, level: 'error' })

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('level=error'),
        expect.any(Object)
      )
    })

    it('should create script with correct payload', async () => {
      const createRequest = {
        name: 'new-script',
        description: 'Test script',
        content: 'log.info("test");',
        enabled: true,
        triggers: [
          {
            trigger_type: 'on_publish' as const,
            topic_filter: '#',
            priority: 100,
            enabled: true,
          },
        ],
      }

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ id: 1, ...createRequest, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z', provisioned_from_config: false }),
      })

      const result = await api.createScript(createRequest)

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/scripts'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(createRequest),
        })
      )

      expect(result).toHaveProperty('id')
      expect(result.name).toBe('new-script')
    })

    it('should update script with correct payload', async () => {
      const updateRequest = {
        name: 'updated-script',
        description: 'Updated',
        content: 'log.info("updated");',
        enabled: false,
        triggers: [],
      }

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ id: 1, ...updateRequest, created_at: '2025-01-01T00:00:00Z', updated_at: '2025-01-01T00:00:00Z', provisioned_from_config: false }),
      })

      await api.updateScript(1, updateRequest)

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/scripts/1'),
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(updateRequest),
        })
      )
    })

    it('should test script with correct event data', async () => {
      const testRequest = {
        content: 'log.info(event.topic);',
        trigger_type: 'on_publish',
        event_data: {
          topic: 'test/topic',
          payload: 'test',
          clientId: 'test-client',
        },
      }

      const mockResponse = {
        success: true,
        logs: [{ level: 'info', message: 'test/topic' }],
        execution_time_ms: 5,
      }

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      })

      const result = await api.testScript(testRequest)

      expect(result.success).toBe(true)
      expect(result.logs).toHaveLength(1)
      expect(result.execution_time_ms).toBe(5)
    })
  })

  describe('Pagination Helpers', () => {
    it('should build query string with pagination params', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: [], pagination: { total: 0, page: 1, page_size: 25, total_pages: 0 } }),
      })

      await api.getMQTTUsers({ page: 2, pageSize: 50, search: 'test', sortBy: 'username', sortOrder: 'desc' })

      const callUrl = mockFetch.mock.calls[0][0] as string
      expect(callUrl).toContain('page=2')
      expect(callUrl).toContain('pageSize=50')
      expect(callUrl).toContain('search=test')
      expect(callUrl).toContain('sortBy=username')
      expect(callUrl).toContain('sortOrder=desc')
    })
  })

  describe('Error Handling', () => {
    it('should throw error on non-ok response', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
        text: async () => '{"error":"not found"}',
      })

      await expect(api.getScript(999)).rejects.toThrow()
    })

    it('should handle network errors', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network error'))

      await expect(api.getScripts()).rejects.toThrow('Network error')
    })
  })
})
