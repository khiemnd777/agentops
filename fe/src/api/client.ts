export const API_BASE = import.meta.env.VITE_API_BASE_URL || '';

export class ApiError extends Error {
  status: number;
  code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) }
  });
  if (!res.ok) {
    const text = await res.text();
    try {
      const parsed = JSON.parse(text);
      const apiError = parsed?.error;
      if (apiError?.message) {
        throw new ApiError(apiError.message, res.status, apiError.code);
      }
    } catch (error) {
      if (error instanceof ApiError) {
        throw error;
      }
    }
    throw new ApiError(text || res.statusText, res.status);
  }
  return res.json();
}

export const get = <T>(path: string) => api<T>(path);
export const post = <T>(path: string, body?: unknown) => api<T>(path, { method: 'POST', body: JSON.stringify(body || {}) });
export const put = <T>(path: string, body?: unknown) => api<T>(path, { method: 'PUT', body: JSON.stringify(body || {}) });
export const del = <T>(path: string) => api<T>(path, { method: 'DELETE' });
